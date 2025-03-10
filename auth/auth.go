package auth

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"osymapp/db"
	"osymapp/utils"

	"github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

var (
	jwtKey            []byte
	blacklistedTokens = struct {
		sync.RWMutex
		tokens map[string]time.Time
	}{tokens: make(map[string]time.Time)}
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Uyarı: .env dosyası yüklenemedi, varsayılan değer kullanılıyor!")
	}
	// JWT anahtarını environment variable'dan al
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		// Eğer environment variable yoksa, geliştirme için sabit değer kullan
		secret = "your-256-bit-secret-key-here-make-it-long-and-secure"
		log.Println("Uyarı: JWT_SECRET environment variable'ı bulunamadı, varsayılan değer kullanılıyor!")
	}
	jwtKey = []byte(secret)

	// Token temizleme işlemini başlat
	go func() {
		for {
			time.Sleep(1 * time.Hour)
			cleanupBlacklistedTokens()
		}
	}()
}

func cleanupBlacklistedTokens() {
	blacklistedTokens.Lock()
	defer blacklistedTokens.Unlock()

	now := time.Now()
	for token, expiry := range blacklistedTokens.tokens {
		if now.After(expiry) {
			delete(blacklistedTokens.tokens, token)
		}
	}
}

func BlacklistToken(tokenString string, expiry time.Time) {
	blacklistedTokens.Lock()
	defer blacklistedTokens.Unlock()
	blacklistedTokens.tokens[tokenString] = expiry
}

func IsTokenBlacklisted(tokenString string) bool {
	blacklistedTokens.RLock()
	defer blacklistedTokens.RUnlock()
	_, exists := blacklistedTokens.tokens[tokenString]
	return exists
}

func GenerateJWT(userID int, username string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  userID,
		"username": username,
		"exp":      time.Now().Add(time.Hour * 24).Unix(), // 24 saat geçerli
		"iat":      time.Now().Unix(),                     // Token oluşturulma zamanı
		"jti":      uuid.New().String(),                   // Benzersiz token ID
	})

	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func GenerateGuestJWT() (string, error) {
	guestID := fmt.Sprintf("guest_%s", uuid.New().String())

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  guestID,
		"username": "guest",
		"is_guest": true,
		"exp":      time.Now().Add(time.Hour * 24).Unix(), // 24 saat geçerli
		"iat":      time.Now().Unix(),
		"jti":      uuid.New().String(),
	})

	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func extractToken(r *http.Request) string {
	tokenString := r.Header.Get("Authorization")
	if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
		return tokenString[7:]
	}

	if cookie, err := r.Cookie("token"); err == nil {
		return cookie.Value
	}

	return ""
}

func JwtVerify(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := extractToken(r)

		if tokenString == "" {
			utils.SendError(w, http.StatusUnauthorized, "Token bulunamadı")
			return
		}

		// Blacklist kontrolü
		if IsTokenBlacklisted(tokenString) {
			utils.SendError(w, http.StatusUnauthorized, "Token geçersiz kılınmış")
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("beklenmeyen imza metodu: %v", token.Header["alg"])
			}
			return jwtKey, nil
		})

		if err != nil {
			utils.SendError(w, http.StatusUnauthorized, "Geçersiz token")
			return
		}

		if !token.Valid {
			utils.SendError(w, http.StatusUnauthorized, "Token geçersiz")
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			utils.SendError(w, http.StatusUnauthorized, "Token içeriği geçersiz")
			return
		}

		// Token süre kontrolü
		if exp, ok := claims["exp"].(float64); ok {
			if time.Now().Unix() > int64(exp) {
				utils.SendError(w, http.StatusUnauthorized, "Token süresi dolmuş")
				return
			}
		}

		// Misafir kullanıcı kontrolü
		isGuest, _ := claims["is_guest"].(bool)
		if isGuest {
			// Misafir kullanıcı için context'e bilgileri ekle
			ctx := context.WithValue(r.Context(), "userID", claims["user_id"])
			ctx = context.WithValue(ctx, "username", "guest")
			ctx = context.WithValue(ctx, "is_guest", true)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Normal kullanıcı işlemleri
		userID := int(claims["user_id"].(float64))
		username := claims["username"].(string)

		// Kullanıcının veritabanında hala aktif olduğunu kontrol et
		pool := db.GetPool()
		var exists bool
		err = pool.QueryRow(context.Background(),
			"SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND username = $2)",
			userID, username).Scan(&exists)

		if err != nil || !exists {
			utils.SendError(w, http.StatusUnauthorized, "Kullanıcı bulunamadı veya pasif durumda")
			return
		}

		// Context'e kullanıcı bilgilerini ekle
		ctx := context.WithValue(r.Context(), "userID", userID)
		ctx = context.WithValue(ctx, "username", username)
		ctx = context.WithValue(ctx, "is_guest", false)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func Logout(w http.ResponseWriter, r *http.Request) {
	tokenString := extractToken(r)
	if tokenString != "" {
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return jwtKey, nil
		})

		if err == nil && token.Valid {
			claims := token.Claims.(jwt.MapClaims)
			if exp, ok := claims["exp"].(float64); ok {
				BlacklistToken(tokenString, time.Unix(int64(exp), 0))
			}
		}

		// Cookie'yi sil
		http.SetCookie(w, &http.Cookie{
			Name:     "token",
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
		})
	}

	utils.SendSuccess(w, "Başarıyla çıkış yapıldı", nil)
}
