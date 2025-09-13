package authservice

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/database-playground/backend-v2/internal/auth"
	"github.com/database-playground/backend-v2/internal/authutil"
	"github.com/database-playground/backend-v2/internal/config"
	"github.com/database-playground/backend-v2/internal/httputils"
	"github.com/database-playground/backend-v2/internal/useraccount"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	googleoauth2 "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

const (
	verifierCookieName = "Gauth-Verifier"
	redirectCookieName = "Gauth-Redirect"
	stateCookieName    = "Gauth-State"
	codeCookieName     = "Gauth-Code"
)

// BuildOAuthConfig builds an oauth2.Config from a gauthConfig.
func BuildOAuthConfig(gauthConfig config.GAuthConfig) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     gauthConfig.ClientID,
		ClientSecret: gauthConfig.ClientSecret,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}
}

type GauthHandler struct {
	oauthConfig  *oauth2.Config
	useraccount  *useraccount.Context
	redirectURIs []string
	secretKey    []byte // AES-256 key for encrypting authorization codes
}

func NewGauthHandler(oauthConfig *oauth2.Config, useraccount *useraccount.Context, redirectURIs []string, secret string) *GauthHandler {
	return &GauthHandler{
		oauthConfig:  oauthConfig,
		useraccount:  useraccount,
		redirectURIs: redirectURIs,
		secretKey:    []byte(secret),
	}
}

// GetByID method for useraccount.Context (assuming it exists)
// If this method doesn't exist, you'll need to implement it or use an alternative approach

// OAuth2Error represents an OAuth 2.0 error response
type OAuth2Error struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
	State            string `json:"state,omitempty"`
}

// OAuth2TokenResponse represents an OAuth 2.0 token response
type OAuth2TokenResponse struct {
	TokenType   string `json:"token_type"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// validatePKCE validates the PKCE code challenge
func validatePKCE(codeChallenge, codeChallengeMethod string) error {
	if codeChallengeMethod != "S256" {
		return errors.New("code_challenge_method must be S256")
	}
	if codeChallenge == "" {
		return errors.New("code_challenge is required")
	}
	return nil
}

// generateCodeVerifier generates a code verifier for PKCE
func generateCodeVerifier() (string, error) {
	return authutil.GenerateToken()
}

// generateCodeChallenge generates a code challenge from a verifier
func generateCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// redirectWithError redirects to the redirect URI with error parameters
func redirectWithError(c *gin.Context, redirectURI, errorCode, errorDescription, state string) {
	if redirectURI == "" {
		// If no redirect URI is available, fall back to JSON response
		c.JSON(http.StatusBadRequest, OAuth2Error{
			Error:            errorCode,
			ErrorDescription: errorDescription,
			State:            state,
		})
		return
	}

	redirectURL, err := url.Parse(redirectURI)
	if err != nil {
		// If redirect URI is invalid, fall back to JSON response
		c.JSON(http.StatusBadRequest, OAuth2Error{
			Error:            "invalid_request",
			ErrorDescription: "Invalid redirect URI",
			State:            state,
		})
		return
	}

	// Add error parameters to query string
	q := redirectURL.Query()
	q.Set("error", errorCode)
	if errorDescription != "" {
		q.Set("error_description", errorDescription)
	}
	if state != "" {
		q.Set("state", state)
	}
	redirectURL.RawQuery = q.Encode()

	c.Redirect(http.StatusFound, redirectURL.String())
}

// Authorize handles the OAuth 2.0 authorization request (RFC 6749 Section 4.1.1)
func (h *GauthHandler) Authorize(c *gin.Context) {
	// Lax since we are using a cookie to store the verifier
	// and the callback will be called by Google (not Strict).
	c.SetSameSite(http.SameSiteLaxMode)

	// Validate required OAuth 2.0 parameters
	responseType := c.Query("response_type")
	redirectURI := c.Query("redirect_uri")
	state := c.Query("state")

	if responseType != "code" {
		redirectWithError(c, redirectURI, "invalid_request", "response_type must be 'code'", state)
		return
	}

	if redirectURI == "" {
		c.JSON(http.StatusBadRequest, OAuth2Error{
			Error:            "invalid_request",
			ErrorDescription: "redirect_uri is required",
			State:            state,
		})
		return
	}

	// Check if redirect URI is allowed first, before other validations
	allowed := slices.Contains(h.redirectURIs, redirectURI)
	if !allowed {
		c.JSON(http.StatusBadRequest, OAuth2Error{
			Error:            "invalid_request",
			ErrorDescription: "Bad redirect URI.",
			State:            state,
		})
		return
	}

	// Validate PKCE parameters (RFC 7636)
	codeChallenge := c.Query("code_challenge")
	codeChallengeMethod := c.Query("code_challenge_method")
	if err := validatePKCE(codeChallenge, codeChallengeMethod); err != nil {
		redirectWithError(c, redirectURI, "invalid_request", err.Error(), state)
		return
	}

	// Generate internal code verifier for Google OAuth
	verifier, err := generateCodeVerifier()
	if err != nil {
		redirectWithError(c, redirectURI, "server_error", "Failed to generate verifier", state)
		return
	}

	callbackURL, err := url.Parse(h.oauthConfig.RedirectURL)
	if err != nil {
		redirectWithError(c, redirectURI, "server_error", "Failed to parse redirect URL", state)
		return
	}

	// Store PKCE parameters and redirect URI in cookies
	c.SetCookie(
		/* name */ verifierCookieName,
		/* value */ verifier,
		/* maxAge */ 5*60, // 5 min
		/* path */ callbackURL.Path,
		/* domain */ "",
		/* secure */ true,
		/* httpOnly */ true,
	)

	c.SetCookie(
		/* name */ redirectCookieName,
		/* value */ redirectURI,
		/* maxAge */ 5*60, // 5 min
		/* path */ "/",
		/* domain */ "",
		/* secure */ true,
		/* httpOnly */ true,
	)

	c.SetCookie(
		/* name */ stateCookieName,
		/* value */ c.Query("state"),
		/* maxAge */ 5*60, // 5 min
		/* path */ "/",
		/* domain */ "",
		/* secure */ true,
		/* httpOnly */ true,
	)

	// Store client's code challenge for later verification
	c.SetCookie(
		/* name */ codeCookieName,
		/* value */ codeChallenge,
		/* maxAge */ 5*60, // 5 min
		/* path */ "/",
		/* domain */ "",
		/* secure */ true,
		/* httpOnly */ true,
	)

	// Generate state for Google OAuth (internal state)
	internalState, err := authutil.GenerateToken()
	if err != nil {
		redirectWithError(c, redirectURI, "server_error", "Failed to generate state", state)
		return
	}

	// Redirect to Google OAuth with PKCE
	googleAuthURL := h.oauthConfig.AuthCodeURL(
		internalState,
		oauth2.AccessTypeOnline,
		oauth2.S256ChallengeOption(verifier),
	)

	c.Redirect(http.StatusFound, googleAuthURL)
}

// Login is kept for backward compatibility, redirects to Authorize
func (h *GauthHandler) Login(c *gin.Context) {
	h.Authorize(c)
}

// AuthorizationCodeData represents the encrypted authorization code payload
type AuthorizationCodeData struct {
	UserID        int       `json:"user_id"`
	RedirectURI   string    `json:"redirect_uri"`
	CodeChallenge string    `json:"code_challenge"`
	State         string    `json:"state"`
	ExpiresAt     time.Time `json:"expires_at"`
}

// encryptAuthCode encrypts authorization code data using AES-256-GCM
func (h *GauthHandler) encryptAuthCode(data *AuthorizationCodeData) (string, error) {
	// Serialize to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal auth code data: %w", err)
	}

	// Create AES cipher
	block, err := aes.NewCipher(h.secretKey)
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt the data
	ciphertext := gcm.Seal(nil, nonce, jsonData, nil)

	// Combine nonce + ciphertext and encode as base64
	encrypted := append(nonce, ciphertext...)
	return base64.URLEncoding.EncodeToString(encrypted), nil
}

// decryptAuthCode decrypts and validates authorization code
func (h *GauthHandler) decryptAuthCode(encryptedCode string) (*AuthorizationCodeData, error) {
	// Decode from base64
	encrypted, err := base64.URLEncoding.DecodeString(encryptedCode)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	// Create AES cipher
	block, err := aes.NewCipher(h.secretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Check minimum length (nonce + at least some ciphertext)
	if len(encrypted) < gcm.NonceSize() {
		return nil, errors.New("encrypted code too short")
	}

	// Extract nonce and ciphertext
	nonce := encrypted[:gcm.NonceSize()]
	ciphertext := encrypted[gcm.NonceSize():]

	// Decrypt the data
	jsonData, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	// Deserialize from JSON
	var data AuthorizationCodeData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal auth code data: %w", err)
	}

	// Check if expired
	if time.Now().After(data.ExpiresAt) {
		return nil, errors.New("authorization code has expired")
	}

	return &data, nil
}

// Callback handles the OAuth callback from Google and generates authorization code for client
func (h *GauthHandler) Callback(c *gin.Context) {
	c.SetSameSite(http.SameSiteStrictMode)

	// Get stored parameters early for error handling
	redirectURI, _ := c.Cookie(redirectCookieName)
	state, _ := c.Cookie(stateCookieName)

	// Get stored verifier for Google OAuth
	verifier, err := c.Cookie(verifierCookieName)
	if err != nil {
		redirectWithError(c, redirectURI, "invalid_request", "Missing verifier cookie", state)
		return
	}

	// Exchange Google authorization code for token
	oauthToken, err := h.oauthConfig.Exchange(c.Request.Context(), c.Query("code"), oauth2.VerifierOption(verifier))
	if err != nil {
		redirectWithError(c, redirectURI, "server_error", "Failed to exchange code with Google", state)
		return
	}

	// Get user info from Google
	client, err := googleoauth2.NewService(
		c.Request.Context(),
		option.WithTokenSource(h.oauthConfig.TokenSource(c.Request.Context(), oauthToken)),
	)
	if err != nil {
		redirectWithError(c, redirectURI, "server_error", "Failed to create Google client", state)
		return
	}

	user, err := client.Userinfo.Get().Do()
	if err != nil {
		redirectWithError(c, redirectURI, "server_error", "Failed to get user info from Google", state)
		return
	}

	// Register or get existing user
	entUser, err := h.useraccount.GetOrRegister(c.Request.Context(), useraccount.UserRegisterRequest{
		Email:  user.Email,
		Name:   user.Name,
		Avatar: user.Picture,
	})
	if err != nil {
		redirectWithError(c, redirectURI, "server_error", "Failed to register user", state)
		return
	}

	// Validate that we have the redirect URI (already retrieved at the beginning)
	if redirectURI == "" {
		c.JSON(http.StatusInternalServerError, OAuth2Error{
			Error:            "server_error",
			ErrorDescription: "Missing redirect URI",
			State:            state,
		})
		return
	}

	codeChallenge, err := c.Cookie(codeCookieName)
	if err != nil {
		redirectWithError(c, redirectURI, "server_error", "Missing code challenge", state)
		return
	}

	// Create authorization code data
	authCodeData := &AuthorizationCodeData{
		UserID:        entUser.ID,
		RedirectURI:   redirectURI,
		CodeChallenge: codeChallenge,
		State:         state,
		ExpiresAt:     time.Now().Add(10 * time.Minute),
	}

	// Encrypt authorization code
	authCode, err := h.encryptAuthCode(authCodeData)
	if err != nil {
		redirectWithError(c, redirectURI, "server_error", "Failed to generate authorization code", state)
		return
	}

	// Clear cookies
	c.SetCookie(verifierCookieName, "", -1, "/", "", true, true)
	c.SetCookie(redirectCookieName, "", -1, "/", "", true, true)
	c.SetCookie(stateCookieName, "", -1, "/", "", true, true)
	c.SetCookie(codeCookieName, "", -1, "/", "", true, true)

	// Redirect to client with authorization code
	redirectURL, err := url.Parse(redirectURI)
	if err != nil {
		redirectWithError(c, redirectURI, "server_error", "Invalid redirect URI", state)
		return
	}

	// Add query parameters
	q := redirectURL.Query()
	q.Set("code", authCode)
	if state != "" {
		q.Set("state", state)
	}
	redirectURL.RawQuery = q.Encode()

	c.Redirect(http.StatusFound, redirectURL.String())
}

// Token handles the OAuth 2.0 token exchange (RFC 6749 Section 4.1.3)
func (h *GauthHandler) Token(c *gin.Context) {
	// Parse form data
	if err := c.Request.ParseForm(); err != nil {
		c.JSON(http.StatusBadRequest, OAuth2Error{
			Error:            "invalid_request",
			ErrorDescription: "Failed to parse form data",
		})
		return
	}

	// Validate grant_type
	grantType := c.Request.FormValue("grant_type")
	if grantType != "authorization_code" {
		c.JSON(http.StatusBadRequest, OAuth2Error{
			Error:            "unsupported_grant_type",
			ErrorDescription: "grant_type must be 'authorization_code'",
		})
		return
	}

	// Get required parameters
	code := c.Request.FormValue("code")
	redirectURI := c.Request.FormValue("redirect_uri")
	codeVerifier := c.Request.FormValue("code_verifier")

	if code == "" {
		c.JSON(http.StatusBadRequest, OAuth2Error{
			Error:            "invalid_request",
			ErrorDescription: "code is required",
		})
		return
	}

	if redirectURI == "" {
		c.JSON(http.StatusBadRequest, OAuth2Error{
			Error:            "invalid_request",
			ErrorDescription: "redirect_uri is required",
		})
		return
	}

	if codeVerifier == "" {
		c.JSON(http.StatusBadRequest, OAuth2Error{
			Error:            "invalid_request",
			ErrorDescription: "code_verifier is required",
		})
		return
	}

	// Decrypt and validate authorization code
	authCodeData, err := h.decryptAuthCode(code)
	if err != nil {
		c.JSON(http.StatusBadRequest, OAuth2Error{
			Error:            "invalid_grant",
			ErrorDescription: "Invalid or expired authorization code",
		})
		return
	}

	// Validate redirect URI
	if authCodeData.RedirectURI != redirectURI {
		c.JSON(http.StatusBadRequest, OAuth2Error{
			Error:            "invalid_grant",
			ErrorDescription: "redirect_uri does not match",
		})
		return
	}

	// Validate PKCE code verifier
	expectedChallenge := generateCodeChallenge(codeVerifier)
	if authCodeData.CodeChallenge != expectedChallenge {
		c.JSON(http.StatusBadRequest, OAuth2Error{
			Error:            "invalid_grant",
			ErrorDescription: "Invalid code verifier",
		})
		return
	}

	// Get user from database
	entUser, err := h.useraccount.GetUser(c.Request.Context(), authCodeData.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, OAuth2Error{
			Error:            "server_error",
			ErrorDescription: "Failed to get user",
		})
		return
	}

	// Generate access token
	machineName := httputils.GetMachineName(c.Request.Context())
	accessToken, err := h.useraccount.GrantToken(c.Request.Context(), entUser, machineName, useraccount.WithFlow("oauth2"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, OAuth2Error{
			Error:            "server_error",
			ErrorDescription: "Failed to generate access token",
		})
		return
	}

	// Return token response
	c.JSON(http.StatusOK, OAuth2TokenResponse{
		TokenType:   "Bearer",
		AccessToken: accessToken,
		ExpiresIn:   auth.DefaultTokenExpire,
	})
}
