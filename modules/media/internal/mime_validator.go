package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"nice-wiki/libs/platform"
)

type SniffAssetTypeInput struct {
	UploadID         Identifier `json:"uploadId"`
	ClaimedMediaType string     `json:"claimedMediaType"`
}

type SniffAssetTypeResult struct {
	MediaType string `json:"mediaType"`
	Animated  bool   `json:"animated"`
	Kind      string `json:"kind"`
	Width     *int   `json:"width"`
	Height    *int   `json:"height"`
}

const maxMetadataBytes = 16 << 20

// SniffAssetType 只读取容器和尺寸元数据，不解码像素、视频帧或执行浏览器内容。
func SniffAssetType(ctx context.Context, op OperationContext, input SniffAssetTypeInput) (SniffAssetTypeResult, error) {
	s, ok := currentBlobStore().(*localBlobStore)
	if !ok {
		return SniffAssetTypeResult{}, platform.NewError(platform.CodeDependencyUnavailable, true, "媒体私有存储未装配")
	}
	record, found, err := readUploadRow(ctx, s.db, input.UploadID, s.validateUpload)
	if err != nil {
		return SniffAssetTypeResult{}, err
	}
	if !found {
		return SniffAssetTypeResult{}, platform.NewError(platform.CodeNotFound, false, "上传不存在")
	}
	reader, err := s.OpenUpload(ctx, input.UploadID)
	if err != nil {
		return SniffAssetTypeResult{}, err
	}
	defer reader.Close()
	file := reader.(*os.File)
	info, err := file.Stat()
	if err != nil {
		return SniffAssetTypeResult{}, mapMediaDBError(err, "读取上传大小失败")
	}
	result := SniffAssetTypeResult{Kind: string(record.session.Kind)}
	// 附件始终以二进制下载交付，文件扩展和浏览器声明都不授予执行能力。
	if result.Kind == "attachment" {
		result.MediaType = "application/octet-stream"
		return result, nil
	}
	head := make([]byte, 512)
	n, err := file.ReadAt(head, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return result, mapMediaDBError(err, "读取上传签名失败")
	}
	head = head[:n]
	ext := strings.ToLower(filepath.Ext(record.session.OriginalName))
	if result.Kind == "drawing_scene" {
		if ext != ".excalidraw" && ext != ".json" {
			return result, invalidMedia("画板场景须使用excalidraw或json扩展")
		}
		if !bytes.HasPrefix(bytes.TrimSpace(head), []byte("{")) {
			return result, invalidMedia("画板场景不是JSON对象")
		}
		result.MediaType = "application/vnd.excalidraw+json"
		return result, nil
	}
	trimmed := bytes.TrimSpace(head)
	if ext == ".svg" || result.Kind == "drawing_svg" || bytes.HasPrefix(trimmed, []byte("<svg")) || bytes.HasPrefix(trimmed, []byte("<?xml")) {
		if ext != ".svg" || (result.Kind != "image" && result.Kind != "drawing_svg") {
			return result, invalidMedia("SVG签名、类别与扩展不一致")
		}
		result.MediaType = "image/svg+xml"
		return result, nil
	}
	kind := http.DetectContentType(head)
	var width, height int
	switch {
	case kind == "image/png":
		if ext != ".png" {
			return result, invalidMedia("PNG签名与扩展不一致")
		}
		result.MediaType = kind
		width, height, result.Animated, err = inspectPNG(ctx, file, info.Size())
	case kind == "image/jpeg":
		if ext != ".jpg" && ext != ".jpeg" && ext != ".jpe" {
			return result, invalidMedia("JPEG签名与扩展不一致")
		}
		result.MediaType = kind
		var config image.Config
		config, _, err = image.DecodeConfig(io.LimitReader(chunkContextReader{ctx, file}, maxMetadataBytes))
		width, height = config.Width, config.Height
		if err == nil {
			var tail [2]byte
			if _, tailErr := file.ReadAt(tail[:], info.Size()-2); tailErr != nil || tail[0] != 0xff || tail[1] != 0xd9 {
				err = invalidMedia("JPEG缺少结束标记")
			}
		}
	case kind == "image/gif":
		if ext != ".gif" {
			return result, invalidMedia("GIF签名与扩展不一致")
		}
		result.MediaType = kind
		width, height, result.Animated, err = inspectGIF(ctx, file, info.Size())
	case len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WEBP":
		if ext != ".webp" {
			return result, invalidMedia("WebP签名与扩展不一致")
		}
		result.MediaType = "image/webp"
		width, height, result.Animated, err = inspectWebP(ctx, file, info.Size())
	case len(head) >= 12 && string(head[4:8]) == "ftyp":
		var avif, animated bool
		avif, animated, err = inspectBrands(file, info.Size())
		if err != nil {
			break
		}
		if avif {
			if ext != ".avif" && ext != ".avifs" {
				return result, invalidMedia("AVIF签名与扩展不一致")
			}
			result.MediaType = "image/avif"
			result.Animated = animated
			width, height, err = inspectBMFF(ctx, file, info.Size(), true)
		} else {
			if result.Kind != "video" || ext != ".mp4" {
				return result, invalidMedia("MP4签名、类别与扩展不一致")
			}
			result.MediaType = "video/mp4"
			_, _, err = inspectBMFF(ctx, file, info.Size(), false)
		}
	case len(head) >= 4 && bytes.Equal(head[:4], []byte{0x1a, 0x45, 0xdf, 0xa3}):
		if result.Kind != "video" || ext != ".webm" {
			return result, invalidMedia("WebM签名、类别与扩展不一致")
		}
		result.MediaType = "video/webm"
		err = inspectWebM(ctx, file, info.Size())
	default:
		return result, invalidMedia("资源格式不受支持；图片使用PNG/JPEG/GIF/WebP/AVIF，视频使用MP4或WebM")
	}
	if err != nil {
		var problem *platform.Error
		if errors.As(err, &problem) {
			return result, err
		}
		if ctx.Err() != nil {
			return result, contextError(ctx.Err())
		}
		return result, invalidMedia("资源容器或尺寸元数据损坏")
	}
	if result.Kind == "video" {
		if !strings.HasPrefix(result.MediaType, "video/") {
			return result, invalidMedia("视频类别与字节格式不一致")
		}
	} else {
		if !strings.HasPrefix(result.MediaType, "image/") {
			return result, invalidMedia("图片类别与字节格式不一致")
		}
		if width < 1 || height < 1 || width > 100000 || height > 100000 {
			return result, invalidMedia("图片尺寸不受支持")
		}
		result.Width = &width
		result.Height = &height
	}
	return result, nil
}

func invalidMedia(message string) error {
	return platform.NewError(platform.CodeValidation, false, "%s", message)
}

// inspectPNG 有界遍历chunk；APNG按acTL帧声明保留动图标记。
func inspectPNG(ctx context.Context, r io.ReaderAt, size int64) (int, int, bool, error) {
	var width, height int
	var animated, data bool
	count := 0
	for offset := int64(8); offset < size; {
		if err := ctx.Err(); err != nil {
			return 0, 0, false, err
		}
		var header [8]byte
		if _, err := r.ReadAt(header[:], offset); err != nil {
			return 0, 0, false, err
		}
		length := int64(binary.BigEndian.Uint32(header[:4]))
		kind := string(header[4:])
		if length > size-offset-12 || count >= 65536 {
			return 0, 0, false, invalidMedia("PNG块边界不合法")
		}
		count++
		switch kind {
		case "IHDR":
			if offset != 8 || length != 13 {
				return 0, 0, false, invalidMedia("PNG头部不合法")
			}
			var p [13]byte
			if _, err := r.ReadAt(p[:], offset+8); err != nil {
				return 0, 0, false, err
			}
			width = int(binary.BigEndian.Uint32(p[:4]))
			height = int(binary.BigEndian.Uint32(p[4:8]))
			if _, _, err := image.DecodeConfig(io.NewSectionReader(r, 0, size)); err != nil {
				return 0, 0, false, err
			}
		case "acTL":
			if length != 8 || data {
				return 0, 0, false, invalidMedia("PNG动画声明不合法")
			}
			var p [8]byte
			if _, err := r.ReadAt(p[:], offset+8); err != nil {
				return 0, 0, false, err
			}
			animated = binary.BigEndian.Uint32(p[:4]) > 1
		case "IDAT":
			if length > 0 {
				data = true
			}
		case "IEND":
			if length != 0 || !data || offset+12 != size {
				return 0, 0, false, invalidMedia("PNG结尾不合法")
			}
			return width, height, animated, nil
		}
		offset += 12 + length
	}
	return 0, 0, false, invalidMedia("PNG缺少结尾")
}

func inspectGIF(ctx context.Context, r io.ReaderAt, size int64) (int, int, bool, error) {
	var header [13]byte
	if _, err := r.ReadAt(header[:], 0); err != nil {
		return 0, 0, false, err
	}
	width, height := int(binary.LittleEndian.Uint16(header[6:8])), int(binary.LittleEndian.Uint16(header[8:10]))
	offset := int64(13)
	frames := 0
	blocks := 0
	if header[10]&0x80 != 0 {
		offset += int64(3 * (1 << ((header[10] & 7) + 1)))
	}
	for offset < size && blocks < 65536 {
		if err := ctx.Err(); err != nil {
			return 0, 0, false, err
		}
		blocks++
		var b [1]byte
		if _, err := r.ReadAt(b[:], offset); err != nil {
			return 0, 0, false, err
		}
		offset++
		switch b[0] {
		case 0x3b:
			if offset != size || frames == 0 {
				return 0, 0, false, invalidMedia("GIF结尾不合法")
			}
			return width, height, frames > 1, nil
		case 0x21:
			offset++ // 扩展标签后为有界子块。
		case 0x2c:
			var imageHeader [9]byte
			if _, err := r.ReadAt(imageHeader[:], offset); err != nil {
				return 0, 0, false, err
			}
			offset += 9
			frames++
			x, y, w, h := int(binary.LittleEndian.Uint16(imageHeader[:2])), int(binary.LittleEndian.Uint16(imageHeader[2:4])), int(binary.LittleEndian.Uint16(imageHeader[4:6])), int(binary.LittleEndian.Uint16(imageHeader[6:8]))
			if w == 0 || h == 0 || x+w > width || y+h > height {
				return 0, 0, false, invalidMedia("GIF帧尺寸不合法")
			}
			if imageHeader[8]&0x80 != 0 {
				offset += int64(3 * (1 << ((imageHeader[8] & 7) + 1)))
			}
			if _, err := r.ReadAt(b[:], offset); err != nil {
				return 0, 0, false, err
			}
			if b[0] < 2 || b[0] > 8 {
				return 0, 0, false, invalidMedia("GIF编码声明不合法")
			}
			offset++
		default:
			return 0, 0, false, invalidMedia("GIF块类型不合法")
		}
		for {
			if _, err := r.ReadAt(b[:], offset); err != nil {
				return 0, 0, false, err
			}
			offset++
			if b[0] == 0 {
				break
			}
			offset += int64(b[0])
			blocks++
			if offset > size || blocks > 65536 {
				return 0, 0, false, invalidMedia("GIF子块超过边界")
			}
		}
	}
	return 0, 0, false, invalidMedia("GIF块数量超限或缺少结尾")
}

func inspectWebP(ctx context.Context, r io.ReaderAt, size int64) (int, int, bool, error) {
	var header [12]byte
	if _, err := r.ReadAt(header[:], 0); err != nil {
		return 0, 0, false, err
	}
	if int64(binary.LittleEndian.Uint32(header[4:8]))+8 != size {
		return 0, 0, false, invalidMedia("WebP长度不一致")
	}
	width, height := 0, 0
	animated := false
	frames := 0
	payload := false
	count := 0
	for offset := int64(12); offset < size; {
		if err := ctx.Err(); err != nil {
			return 0, 0, false, err
		}
		count++
		if count > 65536 {
			return 0, 0, false, invalidMedia("WebP块数量超限")
		}
		var h [8]byte
		if _, err := r.ReadAt(h[:], offset); err != nil {
			return 0, 0, false, err
		}
		length := int64(binary.LittleEndian.Uint32(h[4:]))
		if length > size-offset-8 {
			return 0, 0, false, invalidMedia("WebP块超出边界")
		}
		var p [10]byte
		read := int64(len(p))
		if length < read {
			read = length
		}
		if _, err := r.ReadAt(p[:read], offset+8); err != nil {
			return 0, 0, false, err
		}
		switch string(h[:4]) {
		case "VP8X":
			if offset != 12 || length != 10 {
				return 0, 0, false, invalidMedia("WebP扩展头不合法")
			}
			animated = p[0]&2 != 0
			width = 1 + int(p[4]) + (int(p[5]) << 8) + (int(p[6]) << 16)
			height = 1 + int(p[7]) + (int(p[8]) << 8) + (int(p[9]) << 16)
		case "VP8 ":
			if length < 10 || p[0]&1 != 0 || !bytes.Equal(p[3:6], []byte{0x9d, 1, 0x2a}) {
				return 0, 0, false, invalidMedia("WebP VP8头不合法")
			}
			if width == 0 {
				width = int(binary.LittleEndian.Uint16(p[6:8]) & 0x3fff)
				height = int(binary.LittleEndian.Uint16(p[8:10]) & 0x3fff)
			}
			payload = true
		case "VP8L":
			if length < 5 || p[0] != 0x2f || p[4]&0xe0 != 0 {
				return 0, 0, false, invalidMedia("WebP无损头不合法")
			}
			if width == 0 {
				bits := binary.LittleEndian.Uint32(p[1:5])
				width = int(bits&0x3fff) + 1
				height = int((bits>>14)&0x3fff) + 1
			}
			payload = true
		case "ANMF":
			if !animated || length < 16 {
				return 0, 0, false, invalidMedia("WebP动画帧不合法")
			}
			frames++
		}
		offset += 8 + length + (length & 1)
		if offset > size {
			return 0, 0, false, invalidMedia("WebP对齐超出边界")
		}
	}
	if (!animated && !payload) || (animated && frames == 0) {
		return 0, 0, false, invalidMedia("WebP缺少图像或动画帧")
	}
	return width, height, animated, nil
}

type mediaBox struct {
	kind       string
	start, end int64
}

func readMediaBox(r io.ReaderAt, offset, end int64) (mediaBox, error) {
	var head [16]byte
	if end-offset < 8 {
		return mediaBox{}, invalidMedia("媒体box头被截断")
	}
	if _, err := r.ReadAt(head[:8], offset); err != nil {
		return mediaBox{}, err
	}
	length := int64(binary.BigEndian.Uint32(head[:4]))
	header := int64(8)
	if length == 1 {
		if end-offset < 16 {
			return mediaBox{}, invalidMedia("媒体扩展box头被截断")
		}
		if _, err := r.ReadAt(head[8:], offset+8); err != nil {
			return mediaBox{}, err
		}
		n := binary.BigEndian.Uint64(head[8:])
		if n > uint64(end-offset) {
			return mediaBox{}, invalidMedia("媒体box长度越界")
		}
		length = int64(n)
		header = 16
	} else if length == 0 {
		length = end - offset
	}
	if length < header || length > end-offset {
		return mediaBox{}, invalidMedia("媒体box边界不合法")
	}
	return mediaBox{string(head[4:8]), offset + header, offset + length}, nil
}

func inspectBrands(r io.ReaderAt, size int64) (bool, bool, error) {
	box, err := readMediaBox(r, 0, size)
	if err != nil {
		return false, false, err
	}
	if box.kind != "ftyp" || box.end-box.start < 8 || box.end-box.start > 4096 || (box.end-box.start)%4 != 0 {
		return false, false, invalidMedia("媒体品牌头不合法")
	}
	data := make([]byte, box.end-box.start)
	if _, err := r.ReadAt(data, box.start); err != nil {
		return false, false, err
	}
	avif, animated, mp4 := false, false, false
	for i := 0; i < len(data); i += 4 {
		if i == 4 {
			continue
		}
		switch string(data[i : i+4]) {
		case "avif":
			avif = true
		case "avis":
			avif = true
			animated = true
		case "isom", "iso2", "iso4", "iso5", "iso6", "mp41", "mp42", "avc1", "M4V ":
			mp4 = true
		}
	}
	if !avif && !mp4 {
		return false, false, invalidMedia("仅支持AVIF或MP4媒体品牌")
	}
	return avif, animated, nil
}

// inspectBMFF 跳过编码载荷，只走登记的容器层与样本描述；未知编码不假定可播放。
func inspectBMFF(ctx context.Context, r io.ReaderAt, size int64, avif bool) (int, int, error) {
	width, height, boxes, codecs := 0, 0, 0, 0
	hasData, hasConfig := false, false
	metadata := int64(0)
	var walk func(int64, int64, int) error
	walk = func(start, end int64, depth int) error {
		if depth > 12 {
			return invalidMedia("媒体容器嵌套超限")
		}
		for offset := start; offset < end; {
			if err := ctx.Err(); err != nil {
				return err
			}
			boxes++
			if boxes > 16384 {
				return invalidMedia("媒体box数量超限")
			}
			box, err := readMediaBox(r, offset, end)
			if err != nil {
				return err
			}
			length := box.end - box.start
			switch box.kind {
			case "mdat", "idat":
				if length > 0 {
					hasData = true
				}
			case "moov", "trak", "mdia", "minf", "stbl", "iprp", "ipco":
				if err := walk(box.start, box.end, depth+1); err != nil {
					return err
				}
			case "meta":
				if length < 4 {
					return invalidMedia("AVIF meta头不完整")
				}
				if err := walk(box.start+4, box.end, depth+1); err != nil {
					return err
				}
			case "ispe":
				if length != 12 {
					return invalidMedia("AVIF尺寸头不完整")
				}
				var p [12]byte
				if _, err := r.ReadAt(p[:], box.start); err != nil {
					return err
				}
				w, h := int(binary.BigEndian.Uint32(p[4:8])), int(binary.BigEndian.Uint32(p[8:]))
				if w > width {
					width = w
				}
				if h > height {
					height = h
				}
			case "av1C":
				if length < 4 {
					return invalidMedia("AV1配置不完整")
				}
				hasConfig = true
			case "stsd":
				if length < 8 {
					return invalidMedia("媒体样本描述不完整")
				}
				var p [8]byte
				if _, err := r.ReadAt(p[:], box.start); err != nil {
					return err
				}
				entries := binary.BigEndian.Uint32(p[4:])
				if entries < 1 || entries > 64 {
					return invalidMedia("媒体样本数量不合法")
				}
				cursor := box.start + 8
				for i := uint32(0); i < entries; i++ {
					entry, err := readMediaBox(r, cursor, box.end)
					if err != nil {
						return err
					}
					isVideo, err := validateMP4SampleEntry(ctx, r, entry)
					if err != nil {
						return err
					}
					if isVideo {
						codecs++
					}
					cursor = entry.end
				}
				if cursor != box.end {
					return invalidMedia("媒体样本描述长度不一致")
				}
			default:
				metadata += length
				if metadata > maxMetadataBytes {
					return invalidMedia("媒体元数据超过上限")
				}
			}
			offset = box.end
		}
		return nil
	}
	if err := walk(0, size, 0); err != nil {
		return 0, 0, err
	}
	if !hasData {
		return 0, 0, invalidMedia("媒体容器缺少有效载荷")
	}
	if avif {
		if width < 1 || height < 1 || !hasConfig {
			return 0, 0, invalidMedia("AVIF缺少尺寸或AV1声明")
		}
	} else if codecs == 0 {
		return 0, 0, invalidMedia("MP4缺少允许的视频样本描述")
	}
	return width, height, nil
}

func readEBMLNumber(r io.ReaderAt, offset, end int64, id bool) (uint64, int64, error) {
	var p [8]byte
	if offset >= end {
		return 0, 0, io.ErrUnexpectedEOF
	}
	if _, err := r.ReadAt(p[:1], offset); err != nil {
		return 0, 0, err
	}
	mask := byte(0x80)
	length := int64(1)
	for mask != 0 && p[0]&mask == 0 {
		mask >>= 1
		length++
	}
	limit := int64(8)
	if id {
		limit = 4
	}
	if mask == 0 || length > limit || length > end-offset {
		return 0, 0, invalidMedia("EBML整数不合法")
	}
	if _, err := r.ReadAt(p[:length], offset); err != nil {
		return 0, 0, err
	}
	value := uint64(p[0])
	if !id {
		value &= uint64(mask - 1)
	}
	for i := int64(1); i < length; i++ {
		value = value<<8 | uint64(p[i])
	}
	return value, length, nil
}

func inspectWebM(ctx context.Context, r io.ReaderAt, size int64) error {
	docType, video, data := false, false, false
	elements := 0
	var walk func(int64, int64, int) error
	walk = func(start, end int64, depth int) error {
		if depth > 8 {
			return invalidMedia("WebM嵌套超限")
		}
		for offset := start; offset < end; {
			if err := ctx.Err(); err != nil {
				return err
			}
			elements++
			if elements > 16384 {
				return invalidMedia("WebM元素数量超限")
			}
			id, n, err := readEBMLNumber(r, offset, end, true)
			if err != nil {
				return err
			}
			length, m, err := readEBMLNumber(r, offset+n, end, false)
			if err != nil {
				return err
			}
			payload := offset + n + m
			unknown := length == (uint64(1)<<(7*m))-1
			if unknown {
				if id != 0x18538067 && id != 0x1f43b675 {
					return invalidMedia("WebM未知长度只允许Segment或Cluster")
				}
				length = uint64(end - payload)
			}
			if length > uint64(end-payload) {
				return invalidMedia("WebM元素越界")
			}
			next := payload + int64(length)
			switch id {
			case 0x1a45dfa3, 0x18538067, 0x1654ae6b:
				if err := walk(payload, next, depth+1); err != nil {
					return err
				}
			case 0x4282:
				if length != 4 {
					return invalidMedia("媒体不是WebM")
				}
				var p [4]byte
				if _, err := r.ReadAt(p[:], payload); err != nil {
					return err
				}
				docType = string(p[:]) == "webm"
			case 0xae:
				isVideo, err := validateWebMTrack(ctx, r, payload, next)
				if err != nil {
					return err
				}
				video = video || isVideo
			case 0x6d80:
				return invalidMedia("WebM不接受压缩或加密轨道")
			case 0x1f43b675:
				if length > 0 {
					data = true
				}
			}
			offset = next
		}
		return nil
	}
	if err := walk(0, size, 0); err != nil {
		return err
	}
	if !docType || !video || !data {
		return invalidMedia("WebM缺少类型、视频轨道或帧容器")
	}
	return nil
}

// validateMP4SampleEntry 以样本条目及对应配置声明确证编码，不解析压缩帧。
func validateMP4SampleEntry(ctx context.Context, r io.ReaderAt, entry mediaBox) (bool, error) {
	video := false
	header := int64(28)
	config := ""
	switch entry.kind {
	case "avc1", "avc3":
		video = true
		header = 78
		config = "avcC"
	case "av01":
		video = true
		header = 78
		config = "av1C"
	case "vp09":
		video = true
		header = 78
		config = "vpcC"
	case "mp4a":
		config = "esds"
	case "Opus":
		config = "dOps"
	default:
		return false, invalidMedia("MP4编码不受支持；使用H.264/AV1/VP9及AAC/Opus")
	}
	if entry.end-entry.start < header {
		return false, invalidMedia("MP4样本条目头不完整")
	}
	if !video {
		var version [2]byte
		if _, err := r.ReadAt(version[:], entry.start+8); err != nil {
			return false, err
		}
		if binary.BigEndian.Uint16(version[:]) != 0 {
			return false, invalidMedia("MP4音频样本条目版本不受支持")
		}
	}
	found := false
	boxes := 0
	for offset := entry.start + header; offset < entry.end; {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		boxes++
		if boxes > 256 {
			return false, invalidMedia("MP4样本配置数量超限")
		}
		box, err := readMediaBox(r, offset, entry.end)
		if err != nil {
			return false, err
		}
		if box.kind == config {
			if found {
				return false, invalidMedia("MP4样本配置重复")
			}
			found = true
			length := box.end - box.start
			if length < 1 || length > 65536 {
				return false, invalidMedia("MP4样本配置超过预算")
			}
			data := make([]byte, length)
			if _, err := r.ReadAt(data, box.start); err != nil {
				return false, err
			}
			switch config {
			case "avcC":
				if len(data) < 7 || data[0] != 1 {
					return false, invalidMedia("H.264配置声明不完整")
				}
			case "av1C":
				if len(data) < 4 || data[0] != 0x81 {
					return false, invalidMedia("AV1配置声明不完整")
				}
			case "vpcC":
				if len(data) < 12 || data[0] != 1 {
					return false, invalidMedia("VP9配置声明不完整")
				}
			case "dOps":
				if len(data) < 11 || data[0] != 0 || data[1] == 0 {
					return false, invalidMedia("Opus配置声明不完整")
				}
			case "esds":
				if err := validateAACDescriptor(data); err != nil {
					return false, err
				}
			}
		}
		offset = box.end
	}
	if !found {
		return false, invalidMedia("MP4样本缺少对应的编码配置声明")
	}
	return video, nil
}

func readMP4Descriptor(data []byte) (byte, []byte, []byte, error) {
	if len(data) < 2 {
		return 0, nil, nil, invalidMedia("MP4描述符头不完整")
	}
	tag := data[0]
	length, offset := 0, 1
	for count := 0; count < 4; count++ {
		if offset >= len(data) {
			return 0, nil, nil, invalidMedia("MP4描述符长度被截断")
		}
		value := data[offset]
		offset++
		length = (length << 7) | int(value&0x7f)
		if value&0x80 == 0 {
			if length > len(data)-offset {
				return 0, nil, nil, invalidMedia("MP4描述符长度越界")
			}
			return tag, data[offset : offset+length], data[offset+length:], nil
		}
	}
	return 0, nil, nil, invalidMedia("MP4描述符长度编码超限")
}

// validateAACDescriptor 确认OTI=0x40、音频流类型及AudioSpecificConfig的AAC对象类型。
func validateAACDescriptor(data []byte) error {
	if len(data) < 4 || data[0] != 0 {
		return invalidMedia("AAC esds版本不受支持")
	}
	tag, es, rest, err := readMP4Descriptor(data[4:])
	if err != nil {
		return err
	}
	if tag != 3 || len(es) < 3 || len(rest) != 0 {
		return invalidMedia("AAC缺少完整ES描述符")
	}
	flags := es[2]
	offset := 3
	if flags&0x80 != 0 {
		offset += 2
	}
	if flags&0x40 != 0 {
		if offset >= len(es) {
			return invalidMedia("AAC URL描述符被截断")
		}
		offset += 1 + int(es[offset])
		return invalidMedia("AAC不接受外部URL描述符")
	}
	if flags&0x20 != 0 {
		offset += 2
	}
	if offset > len(es) {
		return invalidMedia("AAC ES描述符被截断")
	}
	found := false
	for children := es[offset:]; len(children) > 0; {
		child, payload, next, err := readMP4Descriptor(children)
		if err != nil {
			return err
		}
		children = next
		if child != 4 {
			continue
		}
		if found || len(payload) < 13 || payload[0] != 0x40 || payload[1]>>2 != 5 {
			return invalidMedia("mp4a不是已支持的MPEG-4 AAC音频声明")
		}
		found = true
		configured := false
		for descriptors := payload[13:]; len(descriptors) > 0; {
			config, audio, tail, err := readMP4Descriptor(descriptors)
			if err != nil {
				return err
			}
			descriptors = tail
			if config == 5 {
				if configured {
					return invalidMedia("AAC音频配置重复")
				}
				configured = true
				if err := validateAACConfig(audio); err != nil {
					return err
				}
			}
		}
		if !configured {
			return invalidMedia("AAC缺少AudioSpecificConfig")
		}
	}
	if !found {
		return invalidMedia("mp4a缺少AAC解码配置")
	}
	return nil
}

func validateAACConfig(data []byte) error {
	position := 0
	read := func(bits int) (uint32, bool) {
		if position+bits > len(data)*8 {
			return 0, false
		}
		value := uint32(0)
		for i := 0; i < bits; i++ {
			value = value<<1 | uint32((data[position/8]>>uint(7-position%8))&1)
			position++
		}
		return value, true
	}
	object, ok := read(5)
	if !ok || (object != 2 && object != 5 && object != 29) {
		return invalidMedia("AAC对象类型仅支持LC/HE/HEv2")
	}
	frequency, ok := read(4)
	if !ok || frequency == 13 || frequency == 14 {
		return invalidMedia("AAC采样频率声明不合法")
	}
	if frequency == 15 {
		value, ok := read(24)
		if !ok || value == 0 {
			return invalidMedia("AAC显式采样频率不合法")
		}
	}
	channels, ok := read(4)
	if !ok || channels < 1 || channels > 7 {
		return invalidMedia("AAC声道配置不受支持")
	}
	if object == 5 || object == 29 {
		frequency, ok = read(4)
		if !ok || frequency == 13 || frequency == 14 {
			return invalidMedia("HE-AAC采样频率声明不合法")
		}
		if frequency == 15 {
			value, ok := read(24)
			if !ok || value == 0 {
				return invalidMedia("HE-AAC显式采样频率不合法")
			}
		}
		core, ok := read(5)
		if !ok || core != 2 {
			return invalidMedia("HE-AAC核心对象类型不受支持")
		}
	}
	return nil
}

func validateWebMTrack(ctx context.Context, r io.ReaderAt, start, end int64) (bool, error) {
	trackType := uint64(0)
	codec := ""
	elements := 0
	if end-start > maxMetadataBytes {
		return false, invalidMedia("WebM轨道元数据超过预算")
	}
	for offset := start; offset < end; {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		elements++
		if elements > 256 {
			return false, invalidMedia("WebM轨道元素数量超限")
		}
		id, n, err := readEBMLNumber(r, offset, end, true)
		if err != nil {
			return false, err
		}
		length, m, err := readEBMLNumber(r, offset+n, end, false)
		if err != nil {
			return false, err
		}
		payload := offset + n + m
		if length > uint64(end-payload) {
			return false, invalidMedia("WebM轨道元素越界")
		}
		switch id {
		case 0x83:
			if trackType != 0 || length < 1 || length > 8 {
				return false, invalidMedia("WebM TrackType重复或长度不合法")
			}
			p := make([]byte, length)
			if _, err := r.ReadAt(p, payload); err != nil {
				return false, err
			}
			for _, value := range p {
				trackType = trackType<<8 | uint64(value)
			}
		case 0x86:
			if codec != "" || length < 1 || length > 32 {
				return false, invalidMedia("WebM CodecID重复或长度不合法")
			}
			p := make([]byte, length)
			if _, err := r.ReadAt(p, payload); err != nil {
				return false, err
			}
			codec = string(p)
		case 0x6d80:
			return false, invalidMedia("WebM不接受压缩或加密轨道")
		}
		offset = payload + int64(length)
	}
	switch trackType {
	case 1:
		if codec != "V_VP8" && codec != "V_VP9" && codec != "V_AV1" {
			return false, invalidMedia("WebM视频轨仅支持VP8/VP9/AV1")
		}
		return true, nil
	case 2:
		if codec != "A_OPUS" && codec != "A_VORBIS" {
			return false, invalidMedia("WebM音频轨仅支持Opus/Vorbis")
		}
		return false, nil
	default:
		return false, invalidMedia("WebM轨道类型不受支持或未声明")
	}
}
