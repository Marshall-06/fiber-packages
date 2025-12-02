package controllers

import (
	"context"
	"encoding/json"
	"errors" 
	"net/http"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"gorm.io/gorm"

	"nazarly-digital/config"
	"nazarly-digital/models"
)

var googleOauthConfig *oauth2.Config

func InitGoogle(clientID, clientSecret, redirectURL string) {
	googleOauthConfig = &oauth2.Config{
		RedirectURL:  redirectURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}
}

func GoogleLogin(c *fiber.Ctx) error {
	if googleOauthConfig == nil {
		return c.Status(500).SendString("oauth not configured")
	}
	state := uuid.NewString()
	c.Cookie(&fiber.Cookie{
		Name:     "oauthstate",
		Value:    state,
		HTTPOnly: true,
		MaxAge:   300, 
	})
	url := googleOauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
	return c.Redirect(url, http.StatusTemporaryRedirect)
}

func GoogleCallback(jwtSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if googleOauthConfig == nil {
			return c.Status(500).SendString("oauth not configured")
		}
		state := c.Query("state")
		stored := c.Cookies("oauthstate")
		if state == "" || stored == "" || state != stored {
			return c.Status(400).JSON(fiber.Map{"error": "invalid oauth state"})
		}

		code := c.Query("code")
		if code == "" {
			return c.Status(400).JSON(fiber.Map{"error": "code missing"})
		}

		tok, err := googleOauthConfig.Exchange(context.Background(), code)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "token exchange failed", "detail": err.Error()})
		}

		client := googleOauthConfig.Client(context.Background(), tok)
		res, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "failed get userinfo", "detail": err.Error()})
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return c.Status(400).JSON(fiber.Map{"error": "userinfo status not OK"})
		}

		var gu map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&gu); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "decode failed", "detail": err.Error()})
		}

		googleID, _ := gu["id"].(string)
		email, _ := gu["email"].(string)
		name, _ := gu["name"].(string)
		picture, _ := gu["picture"].(string)

		if googleID == "" || email == "" {
			return c.Status(400).JSON(fiber.Map{"error": "incomplete google data"})
		}

		var user models.User
		db := config.DB
		if db == nil {
			return c.Status(500).JSON(fiber.Map{"error": "db not initialized"})
		}

		err = db.Where("google_id = ?", googleID).First(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err2 := db.Where("email = ?", email).First(&user).Error
			if errors.Is(err2, gorm.ErrRecordNotFound) {
				user = models.User{
					Email:    email,
					Name:     name,
					Picture:  picture,
					GoogleID: googleID,
					Provider: "google",
				}
				if err := db.Create(&user).Error; err != nil {
					return c.Status(500).JSON(fiber.Map{"error": "create user failed", "detail": err.Error()})
				}
			} else if err2 == nil {
				user.GoogleID = googleID
				user.Provider = "google"
				if err := db.Save(&user).Error; err != nil {
					return c.Status(500).JSON(fiber.Map{"error": "update user failed", "detail": err.Error()})
				}
			} else {
				return c.Status(500).JSON(fiber.Map{"error": "db error", "detail": err2.Error()})
			}
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(500).JSON(fiber.Map{"error": "db error", "detail": err.Error()})
		}

		secret := jwtSecret
		if secret == "" {
			secret = os.Getenv("JWT_SECRET")
		}
		claims := jwt.MapClaims{
			"user_id": user.ID,
			"email":   user.Email,
			"exp":     time.Now().Add(24 * time.Hour).Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signed, err := token.SignedString([]byte(secret))
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "token sign failed", "detail": err.Error()})
		}

		return c.JSON(fiber.Map{
			"token": signed,
			"user":  user,
		})
	}
}
