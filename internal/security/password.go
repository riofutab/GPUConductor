package security

import "golang.org/x/crypto/bcrypt"

// HashPassword 将明文密码转换为bcrypt哈希
func HashPassword(password string) (string, error) {
	if password == "" {
		return "", nil
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

// CheckPassword 比较哈希与明文密码
func CheckPassword(hashed, plain string) bool {
	if hashed == "" || plain == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain)) == nil
}
