package middleware

import (
	"context"
	"net/http"
	"github.com/ifuaslaerl/Judge/internal/auth"
)

// Context key to avoid collisions
type contextKey string

const UserIDKey contextKey = "userID"

// AuthMiddleware verifies the session cookie before allowing access
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Check for cookie
		cookie, err := r.Cookie("session_token")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// 2. Validate token against DB
		userID, ok := auth.GetUserFromSession(cookie.Value)
		if !ok {
			// Invalid/Expired token: Clear cookie and redirect
			http.SetCookie(w, &http.Cookie{Name: "session_token", MaxAge: -1})
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// 3. Inject UserID into context for the next handler
		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		next(w, r.WithContext(ctx))
	}
}
