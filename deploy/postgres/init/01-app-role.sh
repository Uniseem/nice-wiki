#!/bin/sh
# 首次启动数据库时运行：建应用角色（非超级用户、不绕过行级安全）、授权、装 pgvector。
set -eu
psql -v ON_ERROR_STOP=1 -U postgres -d nicewiki <<SQL
CREATE ROLE nw_app LOGIN NOSUPERUSER NOBYPASSRLS PASSWORD '$(cat /run/secrets/pg-app-password)';
GRANT CONNECT, CREATE ON DATABASE nicewiki TO nw_app;
ALTER SCHEMA public OWNER TO nw_app;
CREATE EXTENSION IF NOT EXISTS vector SCHEMA public;
SQL
