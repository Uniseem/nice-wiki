module nice-wiki/modules/traffic-guard

go 1.25.0

require (
	github.com/getkin/kin-openapi v0.133.0
	github.com/oapi-codegen/runtime v1.1.2
	nice-wiki/libs/platform v0.0.0
)

replace nice-wiki/libs/platform => ../../libs/platform
