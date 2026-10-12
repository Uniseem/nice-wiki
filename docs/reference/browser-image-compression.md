# browser-image-compression

> 在浏览器里压缩图片的库，支持 Web Worker。

| 项目 | 信息 |
|---|---|
| 方向 | 编辑器 |
| 仓库 | Donaldcwl/browser-image-compression |
| 许可证 | MIT |
| 主要语言 | JavaScript |
| Star | 1.7k |
| 最近更新 | 2024-03 |

## 它是什么

上传前在浏览器里缩小分辨率或压缩体积。

## 和我们相关的部分

- 支持 jpeg、png、webp、bmp；可以设置最大边长（`maxWidthOrHeight`）、最大体积；可以在 Web Worker 里运行，不卡界面。

## 借鉴什么

- "上传图片时在浏览器里生成多种尺寸"（D26）可以直接用它，或者参考它的实现自己写。

## 注意

- 最近一次更新在 2024 年。浏览器原生的 `OffscreenCanvas` 加 `convertToBlob` 也能做到，实现时再比较。

## 资料

- [仓库](https://github.com/Donaldcwl/browser-image-compression)
