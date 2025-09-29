/**
 * @Author: carlo carlo@paeony.org
 * @Date: 2025-09-29 18:43:56
 * @LastEditors: carlo carlo@paeony.org
 * @LastEditTime: 2025-09-29 18:58:30
 * @FilePath: web/embed.go
 * @Description: 这是默认设置,可以在设置》工具》File Description中进行配置
 */
package web

import (
	"embed"
)

//go:embed dist/*
var StaticFiles embed.FS

//go:embed dist/index.html
var IndexHTML embed.FS
