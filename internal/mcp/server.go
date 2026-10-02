package mcp

import (
	"database/sql"
	"net/http"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const instructions = "有处是家庭物品目录。查询可以直接做。改数据、归类、放入回收站必须是用户刚刚明确要求的操作。不要把对话里的口头答应当成授权。归类只增加 AI 分类，不去掉人工分类。永久删除只能在网页上进行。"

type Server struct {
	db        *sql.DB
	now       func() time.Time
	dataDir   string
	mcpServer *sdk.Server
	handler   http.Handler
}

func New(db *sql.DB, now func() time.Time, dataDir string) http.Handler {
	s := &Server{db: db, now: now, dataDir: dataDir}
	mcpServer := sdk.NewServer(&sdk.Implementation{Name: "youchu", Version: "0.1.0"}, &sdk.ServerOptions{
		Instructions: instructions,
	})
	s.mcpServer = mcpServer
	s.registerReadTools(mcpServer)
	s.registerOrganizeTools(mcpServer)
	s.registerWriteTools(mcpServer)
	s.handler = sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server {
		return mcpServer
	}, &sdk.StreamableHTTPOptions{Stateless: true})
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}
