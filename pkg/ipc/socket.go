package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"
)

// Message IPC消息
type Message struct {
	Type      string                 `json:"type"`      // 消息类型: reload_cache, cleanup_done
	Timestamp time.Time              `json:"timestamp"` // 消息时间
	Data      map[string]interface{} `json:"data"`      // 附加数据
}

// Server Unix Socket服务器
type Server struct {
	socketPath string
	listener   net.Listener
	handlers   map[string]func(Message)
	stopChan   chan struct{}
}

// NewServer 创建新的Socket服务器
func NewServer(socketPath string) (*Server, error) {
	// 删除已存在的socket文件
	os.Remove(socketPath)

	return &Server{
		socketPath: socketPath,
		handlers:   make(map[string]func(Message)),
		stopChan:   make(chan struct{}),
	}, nil
}

// Start 启动Socket服务器
func (s *Server) Start() error {
	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("启动Socket服务器失败: %w", err)
	}

	s.listener = listener

	// 设置权限，允许所有用户访问
	if err := os.Chmod(s.socketPath, 0666); err != nil {
		return fmt.Errorf("设置Socket权限失败: %w", err)
	}

	log.Printf("IPC服务器已启动: %s", s.socketPath)

	go s.acceptLoop()
	return nil
}

// acceptLoop 接受连接循环
func (s *Server) acceptLoop() {
	for {
		select {
		case <-s.stopChan:
			return
		default:
		}

		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.stopChan:
				return
			default:
				log.Printf("接受连接失败: %v", err)
				continue
			}
		}

		go s.handleConnection(conn)
	}
}

// handleConnection 处理连接
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	// 设置超时
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var msg Message
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			log.Printf("解析消息失败: %v", err)
			continue
		}

		// 调用处理器
		if handler, exists := s.handlers[msg.Type]; exists {
			handler(msg)
		} else {
			log.Printf("未知消息类型: %s", msg.Type)
		}
	}
}

// OnMessage 注册消息处理器
func (s *Server) OnMessage(msgType string, handler func(Message)) {
	s.handlers[msgType] = handler
}

// Stop 停止服务器
func (s *Server) Stop() {
	close(s.stopChan)
	if s.listener != nil {
		s.listener.Close()
	}

	// 删除socket文件
	os.Remove(s.socketPath)
}

// Client Unix Socket客户端
type Client struct {
	socketPath string
}

// NewClient 创建新的Socket客户端
func NewClient(socketPath string) *Client {
	return &Client{
		socketPath: socketPath,
	}
}

// SendMessage 发送消息
func (c *Client) SendMessage(msg Message) error {
	conn, err := net.DialTimeout("unix", c.socketPath, 2*time.Second)
	if err != nil {
		return fmt.Errorf("连接Socket服务器失败: %w", err)
	}
	defer conn.Close()

	// 设置写入超时
	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	if _, err := conn.Write(append(data, '\n')); err != nil {
		return err
	}

	return nil
}

// GetDefaultSocketPath 获取默认socket路径
func GetDefaultSocketPath(storagePath string) string {
	// 如果是文件存储，使用存储路径
	// 如果是数据库存储，使用临时目录
	if storagePath == "" {
		storagePath = os.TempDir()
	}

	// 确保目录存在
	os.MkdirAll(storagePath, 0755)

	return filepath.Join(storagePath, "monitor.sock")
}
