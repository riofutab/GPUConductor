package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTConfig struct {
	Secret                string `yaml:"secret"`
	ExpirationHours       int    `yaml:"expiration_hours"`
	RefreshExpirationDays int    `yaml:"refresh_expiration_days"`
}

type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

type RefreshClaims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

var jwtConfig *JWTConfig

// InitJWT 初始化JWT配置
func InitJWT(config *JWTConfig) {
	jwtConfig = config
}

// GenerateToken 生成访问令牌
func GenerateToken(userID, username, role string) (string, error) {
	if jwtConfig == nil {
		return "", fmt.Errorf("JWT未配置")
	}

	claims := &Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(jwtConfig.ExpirationHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "gpuconductor",
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtConfig.Secret))
}

// GenerateRefreshToken 生成刷新令牌
func GenerateRefreshToken(userID string) (string, error) {
	if jwtConfig == nil {
		return "", fmt.Errorf("JWT未配置")
	}

	claims := &RefreshClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(jwtConfig.RefreshExpirationDays) * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "gpuconductor",
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtConfig.Secret))
}

// ValidateToken 验证访问令牌
func ValidateToken(tokenString string) (*Claims, error) {
	if jwtConfig == nil {
		return nil, fmt.Errorf("JWT未配置")
	}

	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("意外的签名方法: %v", token.Header["alg"])
			}
			return []byte(jwtConfig.Secret), nil
		},
	)

	if err != nil {
		return nil, fmt.Errorf("令牌解析失败: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("无效的令牌")
	}

	return claims, nil
}

// ValidateRefreshToken 验证刷新令牌
func ValidateRefreshToken(tokenString string) (*RefreshClaims, error) {
	if jwtConfig == nil {
		return nil, fmt.Errorf("JWT未配置")
	}

	token, err := jwt.ParseWithClaims(
		tokenString,
		&RefreshClaims{},
		func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("意外的签名方法: %v", token.Header["alg"])
			}
			return []byte(jwtConfig.Secret), nil
		},
	)

	if err != nil {
		return nil, fmt.Errorf("令牌解析失败: %w", err)
	}

	claims, ok := token.Claims.(*RefreshClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("无效的刷新令牌")
	}

	return claims, nil
}
