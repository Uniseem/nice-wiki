//! 入库前的确定性文本处理：从发布正文树取检索原文叶、NFKC 规范化（保留到原文的字节偏移）、
//! 按嵌入模型的 token 边界切块、全文检索的词项、背景短句的分批与参考原文。

use std::collections::{HashMap, HashSet};

use serde_json::Value;
use unicode_normalization::char::canonical_combining_class;
use unicode_normalization::UnicodeNormalization;

use nw_platform::platform::{canonical, sha256_hex, Problem};

use crate::types::{BlockRef, Chunk, DocRef};

/// 同一节相邻短块合并到大约这个字节数为止（约一个 512 token 分块），更长的由分块按 token 再切。
const MERGED_LEAF_BYTES: usize = 1200;

pub struct Leaf {
    pub block: BlockRef,
    pub path: String,
    pub text: String,
}

fn integrity(detail: &str) -> Problem {
    Problem::new("INTEGRITY_ERROR", detail)
}

/// 检索原文叶，按文档顺序：同一稳定块里的文字（被加粗、链接拆开的文本节点，图片替代文字、视频题名简介）合成一段；
/// 同一节（到下一个标题为止）里相邻的短块再合并到约 1200 字节，文字用换行连接，以第一个块为引用锚点。
pub fn collect_leaves(body: &Value, document: &DocRef) -> Result<Vec<Leaf>, Problem> {
    let mut pieces: Vec<Leaf> = Vec::new();
    let mut headings = HashSet::new();
    walk(body, "", None, document, &mut pieces, &mut headings)?;
    struct Block {
        block: BlockRef,
        first: String,
        last: String,
        text: String,
        heading: bool,
    }
    let mut blocks: Vec<Block> = Vec::new();
    for piece in pieces {
        if let Some(last) = blocks.last_mut().filter(|b| b.block.block_id == piece.block.block_id) {
            last.text.push_str(&piece.text);
            last.last = piece.path;
            continue;
        }
        let heading = headings.contains(&piece.block.block_id);
        blocks.push(Block { block: piece.block, first: piece.path.clone(), last: piece.path, text: piece.text, heading });
    }
    let mut leaves = Vec::new();
    let mut group: Option<(Leaf, String)> = None;
    for block in blocks {
        if let Some((leaf, _)) = &group {
            if block.heading || leaf.text.len() + 1 + block.text.len() > MERGED_LEAF_BYTES {
                let (mut leaf, last) = group.take().expect("刚判断过非空");
                leaf.path = format!("group:{}..{}", leaf.path, last);
                leaves.push(leaf);
            }
        }
        match &mut group {
            None => group = Some((Leaf { block: block.block, path: block.first, text: block.text }, block.last)),
            Some((leaf, last)) => {
                leaf.text.push('\n');
                leaf.text.push_str(&block.text);
                *last = block.last;
            }
        }
    }
    if let Some((mut leaf, last)) = group {
        leaf.path = format!("group:{}..{}", leaf.path, last);
        leaves.push(leaf);
    }
    Ok(leaves)
}

fn walk(node: &Value, path: &str, current: Option<&BlockRef>, document: &DocRef, out: &mut Vec<Leaf>, headings: &mut HashSet<String>) -> Result<(), Problem> {
    let attrs = &node["attrs"];
    let own;
    let mut current = current;
    if let Some(id) = attrs["blockId"].as_str() {
        own = BlockRef { document: document.clone(), block_id: id.to_owned(), block_hash: sha256_hex(&canonical(node)) };
        current = Some(&own);
        if node["type"] == "heading" {
            headings.insert(id.to_owned());
        }
    }
    let push = |out: &mut Vec<Leaf>, value: &Value, suffix: &str| -> Result<(), Problem> {
        let Some(text) = value.as_str().filter(|t| !t.is_empty()) else { return Ok(()) };
        let block = current.ok_or_else(|| integrity("原叶缺稳定块"))?;
        if text.len() > 4 << 20 {
            return Err(integrity("原叶超过分词预算"));
        }
        out.push(Leaf { block: block.clone(), path: format!("{path}{suffix}"), text: text.to_owned() });
        Ok(())
    };
    push(out, &node["text"], "/text")?;
    if let Some(content) = node["content"].as_array() {
        for (index, child) in content.iter().enumerate() {
            if !child.is_object() {
                return Err(integrity("原正文节点不可读"));
            }
            walk(child, &format!("{path}/content/{index}"), current, document, out, headings)?;
        }
    }
    push(out, &attrs["alt"], "/attrs/alt")?;
    if node["type"] == "video" {
        push(out, &attrs["title"], "/attrs/title")?;
        push(out, &attrs["description"], "/attrs/description")?;
    }
    Ok(())
}

/// 换行统一为 LF 后按稳定边界做 NFKC；每个输出字节记下它所在片段的最早原文字节偏移，引用位置不丢。
pub fn normalize(source: &str) -> (String, Vec<i64>) {
    let mut chars: Vec<(char, i64)> = Vec::with_capacity(source.len());
    let mut iter = source.char_indices().peekable();
    while let Some((at, c)) = iter.next() {
        if c == '\r' {
            if matches!(iter.peek(), Some((_, '\n'))) {
                iter.next();
            }
            chars.push(('\n', at as i64));
        } else {
            chars.push((c, at as i64));
        }
    }
    let mut normalized = String::with_capacity(source.len());
    let mut offsets = Vec::with_capacity(source.len());
    let mut start = 0;
    while start < chars.len() {
        let mut end = start + 1;
        while end < chars.len() && !boundary_before(chars[end].0) {
            end += 1;
        }
        let segment: String = chars[start..end].iter().map(|(c, _)| *c).collect();
        let origin = chars[start..end].iter().map(|(_, at)| *at).min().expect("片段非空");
        let folded: String = segment.nfkc().collect();
        offsets.extend(std::iter::repeat(origin).take(folded.len()));
        normalized.push_str(&folded);
        start = end;
    }
    (normalized, offsets)
}

/// 规范化片段的起点：组合类为 0 的字符（谚文元音、韵尾会与前面的字母组合，不算起点）。
fn boundary_before(c: char) -> bool {
    canonical_combining_class(c) == 0 && !('\u{1161}'..='\u{11C2}').contains(&c)
}

/// 一段规范化后的原文叶，带嵌入模型分词器给出的每个 token 的排他字节终点。
pub struct Source {
    pub leaf_block: BlockRef,
    pub path: String,
    pub source_end: i64,
    pub normalized: String,
    pub offsets: Vec<i64>,
    pub token_ends: Vec<usize>,
    pub context_hash: String,
}

/// 按 token 切块（不跨原文叶），相邻块重叠 overlap 个 token，每块不超过 64 KiB 且落在完整 UTF-8 边界上。
pub fn split(document: &DocRef, sources: &[Source], chunk_tokens: usize, overlap: usize) -> Result<Vec<Chunk>, Problem> {
    let mut chunks = Vec::new();
    for source in sources {
        let text = source.normalized.as_bytes();
        let ends = &source.token_ends;
        if ends.is_empty() {
            continue;
        }
        let boundary = |at: usize| at == text.len() || source.normalized.is_char_boundary(at);
        let mut first = 0;
        while first < ends.len() {
            let start = if first == 0 { 0 } else { ends[first - 1] };
            let mut last = (first + chunk_tokens).min(ends.len());
            while last > first && !(ends[last - 1] - start <= 65536 && boundary(ends[last - 1])) {
                last -= 1;
            }
            if last == first {
                return Err(Problem::new("VALIDATION", "模型分词无法在分块字节预算内保持完整UTF8"));
            }
            let end = ends[last - 1];
            let body = &source.normalized[start..end];
            let end_offset = source.offsets[end - 1];
            let byte_end = source.offsets[end..].iter().copied().find(|o| *o > end_offset).unwrap_or(source.source_end);
            let text_hash = sha256_hex(body.as_bytes());
            let identity = serde_json::json!([source.leaf_block, source.path, source.context_hash, start, end, text_hash]);
            chunks.push(Chunk {
                chunk_id: format!("chunk.{}", sha256_hex(&canonical(&identity))),
                document: document.clone(),
                block: source.leaf_block.clone(),
                ordinal: chunks.len() as i64,
                chunk_hash: text_hash,
                text: body.to_owned(),
                token_count: (last - first) as i64,
                byte_start: source.offsets[start],
                byte_end,
                source_offsets: source.offsets[start..end].to_vec(),
                source_path: Some(source.path.clone()),
                context_prefix: None,
                context_version_hash: Some(source.context_hash.clone()),
                embedding_token_count: None,
            });
            if last == ends.len() {
                break;
            }
            let mut next = (first + 1).max(last.saturating_sub(overlap));
            while next < last && !boundary(ends[next - 1]) {
                next += 1;
            }
            first = next;
        }
    }
    Ok(chunks)
}

fn is_cjk(c: char) -> bool {
    matches!(c as u32,
        0x2E80..=0x2FDF | 0x3005 | 0x3007 | 0x3021..=0x3029 | 0x3038..=0x303B | 0x3041..=0x309F | 0x30A1..=0x30FA | 0x30FD..=0x30FF
        | 0x3105..=0x312F | 0x3131..=0x318E | 0x31F0..=0x31FF | 0x3400..=0x4DBF | 0x4E00..=0x9FFF | 0xA960..=0xA97F | 0xAC00..=0xD7A3
        | 0xD7B0..=0xD7FF | 0xF900..=0xFAFF | 0xFF66..=0xFF9D | 0xFFA0..=0xFFDC | 0x1100..=0x11FF | 0x1B000..=0x1B16F
        | 0x20000..=0x3134F)
}

/// 全文检索的词项：字母数字连续成词（保留大小写，代码标识符不被拆），中日韩连续文字取相邻双字，单字片段保留单字。
pub fn terms(text: &str) -> Vec<String> {
    let mut terms = Vec::new();
    let mut word = String::new();
    let mut cjk: Vec<char> = Vec::new();
    fn flush_cjk(cjk: &mut Vec<char>, terms: &mut Vec<String>) {
        if cjk.len() == 1 {
            terms.push(cjk[0].to_string());
        }
        for pair in cjk.windows(2) {
            terms.push(pair.iter().collect());
        }
        cjk.clear();
    }
    for c in text.chars() {
        if is_cjk(c) {
            if !word.is_empty() {
                terms.push(std::mem::take(&mut word));
            }
            cjk.push(c);
        } else if c.is_alphanumeric() || c == '_' || (!word.is_empty() && canonical_combining_class(c) != 0) {
            flush_cjk(&mut cjk, &mut terms);
            word.push(c);
        } else {
            if !word.is_empty() {
                terms.push(std::mem::take(&mut word));
            }
            flush_cjk(&mut cjk, &mut terms);
        }
    }
    if !word.is_empty() {
        terms.push(word);
    }
    flush_cjk(&mut cjk, &mut terms);
    terms
}

/// 词项 → 次数。
pub fn term_counts(text: &str) -> Vec<(String, u32)> {
    let mut counts: HashMap<String, u32> = HashMap::new();
    for term in terms(text) {
        *counts.entry(term).or_default() += 1;
    }
    counts.into_iter().collect()
}

/// 按 Unicode 字符截断。
pub fn clip(text: &str, limit: usize) -> String {
    match text.char_indices().nth(limit) {
        Some((at, _)) => text[..at].to_owned(),
        None => text.to_owned(),
    }
}

/// 按 UTF-8 字节截断，不切开字符。
pub fn clip_bytes(text: &str, limit: usize) -> &str {
    if text.len() <= limit {
        return text;
    }
    let mut cut = limit;
    while !text.is_char_boundary(cut) {
        cut -= 1;
    }
    &text[..cut]
}

/// 网关的输入估算口径（libs/platform 的 EstimateInputTokens）。
pub fn estimate_tokens(bytes: usize, messages: usize) -> usize {
    let base = (bytes + 1) / 2 + messages * 32;
    (base * 6 + 4) / 5
}

// ---------- 背景短句的分批 ----------

/// 一批最多的片段数，目录最多占的字节数。
pub const CONTEXT_BATCH: usize = 16;
const OUTLINE_BYTES: usize = 4000;

pub struct ContextBatch {
    pub chunks: Vec<usize>,
    pub context: String,
}

/// 一批片段发给模型的 JSON。
pub fn batch_json(chunks: &[&Chunk]) -> String {
    let items: Vec<Value> = chunks.iter().map(|c| serde_json::json!({"chunkId": c.chunk_id, "text": c.text})).collect();
    Value::Array(items).to_string()
}

/// 一批片段之外还能放多少字节参考原文：让整次请求留在背景模型窗口的九成以内（四条消息）。
pub fn context_room(window: usize, fixed: usize, batch: &[&Chunk]) -> i64 {
    (window as i64 * 3 / 4 - 4 * 32) * 2 - fixed as i64 - batch_json(batch).len() as i64
}

struct Section {
    heading: String,
    chunks: Vec<usize>,
}

/// 按正文树顶层的标题切节；块不在树里的片段跟随前一个片段。
fn sections(body: &Value, chunks: &[Chunk]) -> (Vec<Section>, Vec<usize>) {
    fn text(node: &Value) -> String {
        let mut out = node["text"].as_str().unwrap_or_default().to_owned();
        for child in node["content"].as_array().into_iter().flatten() {
            out.push_str(&text(child));
        }
        out
    }
    fn mark(node: &Value, index: usize, map: &mut HashMap<String, usize>) {
        if let Some(id) = node["attrs"]["blockId"].as_str() {
            map.insert(id.to_owned(), index);
        }
        for child in node["content"].as_array().into_iter().flatten() {
            mark(child, index, map);
        }
    }
    let mut sections = vec![Section { heading: String::new(), chunks: Vec::new() }];
    let mut block_section = HashMap::new();
    for top in body["content"].as_array().into_iter().flatten() {
        if top["type"] == "heading" {
            let heading = text(top).trim().to_owned();
            if !heading.is_empty() {
                sections.push(Section { heading: clip_bytes(&heading, 200).to_owned(), chunks: Vec::new() });
            }
        }
        mark(top, sections.len() - 1, &mut block_section);
    }
    let mut owner = Vec::with_capacity(chunks.len());
    for (i, chunk) in chunks.iter().enumerate() {
        let index = block_section.get(&chunk.block.block_id).copied().unwrap_or(if i > 0 { owner[i - 1] } else { 0 });
        owner.push(index);
        sections[index].chunks.push(i);
    }
    (sections, owner)
}

/// 把片段按章节分批：连续片段凑成一批（最多 16 段），只要“全文目录 + 涉及章节的原文”放得进 room 给出的字节数；
/// 一节原文本身放不下时，只取这批片段前后相邻的同节片段原文。
pub fn plan_context_batches(body: &Value, chunks: &[Chunk], room: impl Fn(&[&Chunk]) -> i64) -> Vec<ContextBatch> {
    let (sections, owner) = sections(body, chunks);
    let headings: Vec<&str> = sections.iter().filter(|s| !s.heading.is_empty()).map(|s| s.heading.as_str()).collect();
    let outline = if headings.is_empty() { String::new() } else { format!("{}\n\n", clip_bytes(&format!("全文目录：\n{}", headings.join("\n")), OUTLINE_BYTES)) };
    let section_text = |index: usize, from: usize, to: usize| {
        let mut out = String::new();
        if !sections[index].heading.is_empty() {
            out.push_str(&format!("## {}\n", sections[index].heading));
        }
        for &i in &sections[index].chunks[from..to] {
            out.push_str(&chunks[i].text);
            out.push('\n');
        }
        out
    };
    let pick = |batch: &[usize]| -> Vec<&Chunk> { batch.iter().map(|&i| &chunks[i]).collect() };
    let whole = |batch: &[usize]| {
        let mut out = outline.clone();
        let mut seen = HashSet::new();
        for &i in batch {
            if seen.insert(owner[i]) {
                out.push_str(&section_text(owner[i], 0, sections[owner[i]].chunks.len()));
            }
        }
        out
    };
    let context_for = |batch: &[usize]| {
        let limit = room(&pick(batch));
        let full = whole(batch);
        if full.len() as i64 <= limit {
            return full;
        }
        let home = owner[batch[0]];
        let members = &sections[home].chunks;
        let position = |value: usize| members.iter().position(|&v| v == value).unwrap_or(0);
        let (mut from, mut to) = (position(batch[0]), position(batch[batch.len() - 1]) + 1);
        loop {
            let mut grown = false;
            if from > 0 && (outline.len() + section_text(home, from - 1, to).len()) as i64 <= limit {
                from -= 1;
                grown = true;
            }
            if to < members.len() && (outline.len() + section_text(home, from, to + 1).len()) as i64 <= limit {
                to += 1;
                grown = true;
            }
            if !grown {
                break;
            }
        }
        clip_bytes(&format!("{outline}{}", section_text(home, from, to)), limit.max(0) as usize).to_owned()
    };
    let mut batches = Vec::new();
    let mut current: Vec<usize> = Vec::new();
    for i in 0..chunks.len() {
        let mut next = current.clone();
        next.push(i);
        let full = current.len() == CONTEXT_BATCH;
        let new_section = !current.is_empty() && owner[i] != owner[*current.last().expect("非空")] && whole(&next).len() as i64 > room(&pick(&next));
        if !current.is_empty() && (full || new_section || room(&pick(&next)) <= outline.len() as i64) {
            batches.push(ContextBatch { context: context_for(&current), chunks: std::mem::take(&mut current) });
            next = vec![i];
        }
        current = next;
    }
    if !current.is_empty() {
        batches.push(ContextBatch { context: context_for(&current), chunks: current });
    }
    batches
}
