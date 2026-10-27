#!/bin/sh
# 首次启动数据库时运行：建共享角色 nw_shared（公共辅助函数和队列表的所有者），再按 init-secrets.sh 生成的 roles.sql
# 给每个组件建一个角色（非超级用户、不绕过行级安全，是 nw_shared 的成员，只能建和用自己的 schema），装 pgvector。
set -eu
psql -v ON_ERROR_STOP=1 -U postgres -d nicewiki <<SQL
CREATE ROLE nw_shared NOLOGIN NOSUPERUSER NOBYPASSRLS;
GRANT CREATE ON DATABASE nicewiki TO nw_shared;
ALTER SCHEMA public OWNER TO nw_shared;
CREATE EXTENSION IF NOT EXISTS vector SCHEMA public;
SQL
psql -v ON_ERROR_STOP=1 -U postgres -d nicewiki -f /run/secrets/pg-roles.sql
