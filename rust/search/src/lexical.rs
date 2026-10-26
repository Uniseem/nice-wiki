use std::path::Path;
use std::sync::Mutex;

use anyhow::Context;
use tantivy::collector::TopDocs;
use tantivy::query::{BooleanQuery, ConstScoreQuery, Occur, Query, TermQuery};
use tantivy::schema::{Field, IndexRecordOption, Schema, TextFieldIndexing, TextOptions, Value, STORED, STRING};
use tantivy::tokenizer::WhitespaceTokenizer;
use tantivy::{Index, IndexReader, IndexWriter, ReloadPolicy, TantivyDocument, Term};

/// 写入一个分块：词项由调用方分好（词 → 次数），这里按次数重复后用空白分词器入库。
pub struct Entry {
    pub chunk_id: String,
    pub build_id: String,
    pub terms: Vec<(String, u32)>,
}

pub struct Hit {
    pub chunk_id: String,
    pub build_id: String,
    pub document_id: String,
    pub score: f32,
}

struct Fields {
    site: Field,
    doc_key: Field,
    space: Field,
    chunk: Field,
    build: Field,
    document: Field,
    body: Field,
}

pub struct Lexical {
    index: Index,
    reader: IndexReader,
    writer: Mutex<IndexWriter>,
    fields: Fields,
}

fn doc_key(site: &str, document: &str) -> String {
    format!("{site}\u{1f}{document}")
}

impl Lexical {
    pub fn open(dir: &str) -> anyhow::Result<Self> {
        let mut builder = Schema::builder();
        let site = builder.add_text_field("site", STRING);
        let doc_key_field = builder.add_text_field("doc_key", STRING);
        let space = builder.add_text_field("space", STRING);
        let chunk = builder.add_text_field("chunk", STRING | STORED);
        let build = builder.add_text_field("build", STRING | STORED);
        let document = builder.add_text_field("document", STRING | STORED);
        let body = builder.add_text_field(
            "body",
            TextOptions::default().set_indexing_options(TextFieldIndexing::default().set_tokenizer("nw_terms").set_index_option(IndexRecordOption::WithFreqs)),
        );
        let schema = builder.build();
        std::fs::create_dir_all(dir).with_context(|| format!("创建索引目录 {dir} 失败"))?;
        let index = Index::open_or_create(tantivy::directory::MmapDirectory::open(Path::new(dir))?, schema)?;
        index.tokenizers().register("nw_terms", WhitespaceTokenizer::default());
        let writer = index.writer(64 << 20)?;
        let reader = index.reader_builder().reload_policy(ReloadPolicy::Manual).try_into()?;
        Ok(Self { index, reader, writer: Mutex::new(writer), fields: Fields { site, doc_key: doc_key_field, space, chunk, build, document, body } })
    }

    fn commit(&self, writer: &mut IndexWriter) -> anyhow::Result<()> {
        writer.commit()?;
        self.reader.reload()?;
        Ok(())
    }

    /// 整篇文档替换：先删旧的，再写入新分块，一次提交。
    pub fn replace_document(&self, site: &str, document: &str, space: &str, entries: &[Entry]) -> anyhow::Result<()> {
        let mut writer = self.writer.lock().map_err(|_| anyhow::anyhow!("索引写锁损坏"))?;
        writer.delete_term(Term::from_field_text(self.fields.doc_key, &doc_key(site, document)));
        for entry in entries {
            let mut body = String::new();
            for (term, count) in &entry.terms {
                for _ in 0..(*count).min(2000) {
                    body.push_str(term);
                    body.push(' ');
                }
            }
            let mut doc = TantivyDocument::default();
            doc.add_text(self.fields.site, site);
            doc.add_text(self.fields.doc_key, doc_key(site, document));
            doc.add_text(self.fields.space, space);
            doc.add_text(self.fields.chunk, &entry.chunk_id);
            doc.add_text(self.fields.build, &entry.build_id);
            doc.add_text(self.fields.document, document);
            doc.add_text(self.fields.body, &body);
            writer.add_document(doc)?;
        }
        self.commit(&mut writer)
    }

    pub fn delete_document(&self, site: &str, document: &str) -> anyhow::Result<()> {
        let mut writer = self.writer.lock().map_err(|_| anyhow::anyhow!("索引写锁损坏"))?;
        writer.delete_term(Term::from_field_text(self.fields.doc_key, &doc_key(site, document)));
        self.commit(&mut writer)
    }

    pub fn delete_site(&self, site: &str) -> anyhow::Result<()> {
        let mut writer = self.writer.lock().map_err(|_| anyhow::anyhow!("索引写锁损坏"))?;
        writer.delete_term(Term::from_field_text(self.fields.site, site));
        self.commit(&mut writer)
    }

    /// BM25 排名。站点和空间是必须条件（不计分），词项之间是“或”，文档范围可选。
    pub fn search(&self, site: &str, space: &str, terms: &[String], documents: &[String], limit: usize) -> anyhow::Result<Vec<Hit>> {
        if terms.is_empty() || limit == 0 {
            return Ok(Vec::new());
        }
        let filter = |field: Field, value: &str| -> Box<dyn Query> {
            Box::new(ConstScoreQuery::new(Box::new(TermQuery::new(Term::from_field_text(field, value), IndexRecordOption::Basic)), 0.0))
        };
        let mut clauses: Vec<(Occur, Box<dyn Query>)> = vec![(Occur::Must, filter(self.fields.site, site)), (Occur::Must, filter(self.fields.space, space))];
        if !documents.is_empty() {
            let any: Vec<(Occur, Box<dyn Query>)> = documents.iter().map(|d| (Occur::Should, filter(self.fields.doc_key, &doc_key(site, d)))).collect();
            clauses.push((Occur::Must, Box::new(BooleanQuery::new(any))));
        }
        let mut unique: Vec<&String> = terms.iter().collect();
        unique.sort();
        unique.dedup();
        let scored: Vec<(Occur, Box<dyn Query>)> = unique
            .into_iter()
            .map(|term| (Occur::Should, Box::new(TermQuery::new(Term::from_field_text(self.fields.body, term), IndexRecordOption::WithFreqs)) as Box<dyn Query>))
            .collect();
        clauses.push((Occur::Must, Box::new(BooleanQuery::new(scored))));
        let query = BooleanQuery::new(clauses);
        let searcher = self.reader.searcher();
        let top = searcher.search(&query, &TopDocs::with_limit(limit))?;
        let mut hits = Vec::with_capacity(top.len());
        for (score, address) in top {
            let doc: TantivyDocument = searcher.doc(address)?;
            let text = |field: Field| doc.get_first(field).and_then(|v| v.as_str()).unwrap_or_default().to_owned();
            hits.push(Hit { chunk_id: text(self.fields.chunk), build_id: text(self.fields.build), document_id: text(self.fields.document), score });
        }
        Ok(hits)
    }

    pub fn doc_count(&self) -> u64 {
        self.reader.searcher().num_docs()
    }

    #[allow(dead_code)]
    pub fn index(&self) -> &Index {
        &self.index
    }
}
