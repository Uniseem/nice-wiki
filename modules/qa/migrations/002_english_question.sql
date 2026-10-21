-- 每轮提问的英文译文：决策模型的输入一律用英文，后续轮次判断“是否依赖上文”时读最近几轮的英文提问。
ALTER TABLE turns ADD COLUMN english_question TEXT NOT NULL DEFAULT '';
