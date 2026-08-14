// Package config 负责加载并解析面板配置。
//
// 配置来源(优先级从高到低):
//  1. 环境变量(便于 Docker 部署注入);
//  2. config.yaml 文件;
//  3. 内置默认值。
//
// JWT 密钥未配置时自动生成 32 字节随机值并持久化到数据目录,
// 保证重启后已签发的 token 仍然有效。
package config

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	JWT      JWTConfig      `yaml:"jwt"`
	Data     DataConfig     `yaml:"data"`
	Backup   BackupConfig   `yaml:"backup"`
	Security SecurityConfig `yaml:"security"`
	CORS     CORSConfig     `yaml:"cors"`
}

type ServerConfig struct {
	Port   int    `yaml:"port"`
	Mode   string `yaml:"mode"`
	WebDir string `yaml:"web_dir"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type JWTConfig struct {
	Secret        string `yaml:"secret"`
	TokenExpireH  int    `yaml:"token_expire_h"` // 访问令牌有效期(小时)
}

type DataConfig struct {
	Dir        string `yaml:"dir"`
	ScriptsDir string `yaml:"scripts_dir"`
	LogDir     string `yaml:"log_dir"`
}

type BackupConfig struct {
	Dir string `yaml:"dir"`
}

// SecurityConfig 安全相关配置。
type SecurityConfig struct {
	// IPWhitelist 访问白名单:逗号分隔的单个 IP 或 CIDR;为空表示不启用(允许所有来源)。
	IPWhitelist []string `yaml:"ip_whitelist"`
}

type CORSConfig struct {
	Origins []string `yaml:"origins"`
}

// C 是全局配置实例,初始化后可供各包读取。
var C *Config

// Load 从 path 加载配置。相对路径以配置文件所在目录为基准解析。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// 环境变量覆盖
	if v := os.Getenv("SERVER_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = p
		}
	}
	if v := os.Getenv("DB_PATH"); v != "" {
		cfg.Database.Path = v
	}
	if v := os.Getenv("DATA_DIR"); v != "" {
		cfg.Data.Dir = v
	}
	if v := os.Getenv("WEB_DIR"); v != "" {
		cfg.Server.WebDir = v
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.JWT.Secret = v
	}
	if v := os.Getenv("CORS_ORIGINS"); v != "" {
		for _, item := range strings.Split(v, ",") {
			if item = strings.TrimSpace(item); item != "" {
				cfg.CORS.Origins = append(cfg.CORS.Origins, item)
			}
		}
	}
	if v := os.Getenv("IP_WHITELIST"); v != "" {
		for _, item := range strings.Split(v, ",") {
			if item = strings.TrimSpace(item); item != "" {
				cfg.Security.IPWhitelist = append(cfg.Security.IPWhitelist, item)
			}
		}
	}

	// 默认值
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 5700
	}
	if cfg.Server.Mode == "" {
		cfg.Server.Mode = "release"
	}
	if cfg.JWT.TokenExpireH == 0 {
		cfg.JWT.TokenExpireH = 72 // 3 天
	}

	// 路径解析锚点:config.yaml 所在目录,避免 cwd 漂移导致数据写到别处。
	configDir, err := filepath.Abs(filepath.Dir(path))
	if err != nil || configDir == "" {
		configDir, _ = os.Getwd()
	}
	cfg.Data.Dir = resolveDataPath(configDir, cfg.Data.Dir)
	cfg.Data.ScriptsDir = resolveDataPath(configDir, cfg.Data.ScriptsDir)
	cfg.Data.LogDir = resolveDataPath(configDir, cfg.Data.LogDir)
	cfg.Database.Path = resolveDataPath(configDir, cfg.Database.Path)
	cfg.Backup.Dir = resolveDataPath(configDir, cfg.Backup.Dir)

	if cfg.Data.ScriptsDir == "" {
		cfg.Data.ScriptsDir = filepath.Join(cfg.Data.Dir, "scripts")
	}
	if cfg.Data.LogDir == "" {
		cfg.Data.LogDir = filepath.Join(cfg.Data.Dir, "logs")
	}
	if cfg.Backup.Dir == "" {
		cfg.Backup.Dir = filepath.Join(cfg.Data.Dir, "backups")
	}

	for _, dir := range []string{cfg.Data.Dir, cfg.Data.ScriptsDir, cfg.Data.LogDir, cfg.Backup.Dir} {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Printf("warn: create data dir failed: %v", err)
		}
	}

	// JWT 密钥自动生成并持久化
	if cfg.JWT.Secret == "" {
		cfg.JWT.Secret = loadOrGenerateSecret(cfg.Data.Dir)
	}

	C = cfg
	return cfg, nil
}

func resolveDataPath(baseDir, raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "." {
		return trimmed
	}
	if filepath.IsAbs(trimmed) {
		return trimmed
	}
	return filepath.Clean(filepath.Join(baseDir, trimmed))
}

func loadOrGenerateSecret(dataDir string) string {
	secretFile := filepath.Join(dataDir, ".jwt_secret")
	if data, err := os.ReadFile(secretFile); err == nil && len(strings.TrimSpace(string(data))) > 0 {
		return strings.TrimSpace(string(data))
	}

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("生成 JWT 密钥失败: %v", err)
	}
	secret := hex.EncodeToString(b)
	_ = os.MkdirAll(dataDir, 0o755)
	if err := os.WriteFile(secretFile, []byte(secret), 0o600); err != nil {
		log.Printf("warn: 持久化 JWT 密钥失败: %v", err)
	}
	return secret
}

// BackupKey 返回 32 字节的备份加密密钥(hex 持久化到 dataDir/.backup_key)。
// 密钥首次自动生成,重启后保持一致,保证历史备份可恢复。
func BackupKey(dataDir string) []byte {
	keyFile := filepath.Join(dataDir, ".backup_key")
	if data, err := os.ReadFile(keyFile); err == nil && len(strings.TrimSpace(string(data))) > 0 {
		if key, err := hex.DecodeString(strings.TrimSpace(string(data))); err == nil && len(key) == 32 {
			return key
		}
	}

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("生成备份密钥失败: %v", err)
	}
	_ = os.MkdirAll(dataDir, 0o755)
	if err := os.WriteFile(keyFile, []byte(hex.EncodeToString(b)), 0o600); err != nil {
		log.Printf("warn: 持久化备份密钥失败: %v", err)
	}
	return b
}
