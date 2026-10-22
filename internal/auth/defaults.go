package auth

// Built-in Google sign-in. Fill these in once (or set them at build time with
// -ldflags "-X trimmel/internal/auth.GoogleClientID=... -X trimmel/internal/auth.GoogleClientSecret=...")
// and users just click "Sign in with Google": no config files.
//
// Create the values in Google Cloud Console: APIs & Services > Credentials >
// Create credentials > OAuth client ID > Application type "Desktop app".
// Google issues a secret for desktop apps but does not treat it as
// confidential (PKCE protects the flow), so shipping it in the binary is
// the documented approach.
var (
	GoogleClientID     = ""
	GoogleClientSecret = ""
)

const googleIssuer = "https://accounts.google.com"
