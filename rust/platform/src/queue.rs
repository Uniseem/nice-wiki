//! 事件队列的消费端（与 Go 的 libs/platform/queue 共用 queue 模式下的表）：
//! 登记订阅并补发近 7 天的事件，按事件顺序领取到期的投递（租约 45 秒起、每读一次翻倍、最长 5 分钟，即失败退避），
//! 处理成功后确认；读满 10 次进死信。队列表由 Go 服务建好，这里只读写。

use std::future::Future;
use std::time::Duration;

use deadpool_postgres::Pool;

pub struct Event {
    pub event_id: String,
    pub event_type: String,
    pub event_version: i32,
    pub subject_id: String,
    pub revision: i64,
    pub data_epoch: String,
    pub site_id: String,
    pub payload: Vec<u8>,
}

const MAX_ATTEMPTS: i32 = 10;

const CLAIM: &str = "UPDATE queue.deliveries d
SET attempts = d.attempts + 1, visible_at = now() + make_interval(secs => LEAST(300, 45 * power(2, d.attempts))), updated_at = now()
FROM (
    SELECT x.event_id FROM queue.deliveries x JOIN queue.events e ON e.event_id = x.event_id
    WHERE x.consumer = $1 AND x.state = 'queued' AND x.visible_at <= now()
    ORDER BY e.seq LIMIT 1 FOR UPDATE OF x SKIP LOCKED
) due
WHERE d.event_id = due.event_id AND d.consumer = $1
RETURNING d.event_id";

pub struct Consumer {
    pool: Pool,
    name: String,
}

impl Consumer {
    pub fn new(pool: Pool, name: &str) -> Self {
        Self { pool, name: name.to_owned() }
    }

    /// keys 形如 "content.publication.changed@1"。
    pub async fn register(&self, keys: &[&str]) -> anyhow::Result<()> {
        let mut client = self.pool.get().await?;
        let tx = client.transaction().await?;
        for key in keys {
            tx.execute("INSERT INTO queue.subscriptions (consumer, event_key) VALUES ($1, $2) ON CONFLICT DO NOTHING", &[&self.name, key]).await?;
        }
        let keys: Vec<String> = keys.iter().map(|k| k.to_string()).collect();
        tx.execute(
            "INSERT INTO queue.deliveries (event_id, consumer) SELECT e.event_id, $1 FROM queue.events e
             WHERE e.event_type || '@' || e.event_version = ANY($2) AND e.created_at > now() - interval '7 days' ON CONFLICT DO NOTHING",
            &[&self.name, &keys],
        )
        .await?;
        tx.commit().await?;
        Ok(())
    }

    pub async fn claim(&self) -> anyhow::Result<Option<Event>> {
        let client = self.pool.get().await?;
        client
            .execute(
                "UPDATE queue.deliveries SET state = 'dead-letter', last_error = 'attempts exceeded', updated_at = now()
                 WHERE consumer = $1 AND state = 'queued' AND visible_at <= now() AND attempts >= $2",
                &[&self.name, &MAX_ATTEMPTS],
            )
            .await?;
        let Some(row) = client.query_opt(CLAIM, &[&self.name]).await? else { return Ok(None) };
        let event_id: String = row.get(0);
        let row = client
            .query_one("SELECT event_type, event_version, subject_id, revision, data_epoch, site_id, payload FROM queue.events WHERE event_id = $1", &[&event_id])
            .await?;
        Ok(Some(Event {
            event_id,
            event_type: row.get(0),
            event_version: row.get(1),
            subject_id: row.get(2),
            revision: row.get(3),
            data_epoch: row.get(4),
            site_id: row.get(5),
            payload: row.get(6),
        }))
    }

    pub async fn ack(&self, event_id: &str) -> anyhow::Result<()> {
        let client = self.pool.get().await?;
        client.execute("UPDATE queue.deliveries SET state = 'done', updated_at = now() WHERE event_id = $1 AND consumer = $2", &[&event_id, &self.name]).await?;
        Ok(())
    }

    /// 一直消费：处理成功就确认，失败留给租约到期后重读。登记失败（队列表还没建好）时隔几秒重试。
    pub async fn run<F, Fut>(self, keys: &[&str], handle: F)
    where
        F: Fn(Event) -> Fut,
        Fut: Future<Output = anyhow::Result<()>>,
    {
        while let Err(error) = self.register(keys).await {
            eprintln!("{} 登记事件订阅失败，稍后重试：{error:#}", self.name);
            tokio::time::sleep(Duration::from_secs(5)).await;
        }
        loop {
            match self.claim().await {
                Ok(Some(event)) => {
                    let id = event.event_id.clone();
                    let kind = event.event_type.clone();
                    match handle(event).await {
                        Ok(()) => {
                            if let Err(error) = self.ack(&id).await {
                                eprintln!("{} 确认事件失败：{error:#}", self.name);
                            }
                        }
                        Err(error) => eprintln!("{} 处理事件 {kind} 失败，稍后重试：{error:#}", self.name),
                    }
                }
                Ok(None) => tokio::time::sleep(Duration::from_millis(400)).await,
                Err(error) => {
                    eprintln!("{} 读取事件失败：{error:#}", self.name);
                    tokio::time::sleep(Duration::from_secs(2)).await;
                }
            }
        }
    }
}
