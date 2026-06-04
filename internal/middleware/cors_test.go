package middleware

import "testing"

func TestOriginAllowedMatchesExactOrigin(t *testing.T) {
	allowed := parseAllowedOrigins("https://portofolio-ecommerce-footwear.vercel.app")
	if !originAllowed("https://portofolio-ecommerce-footwear.vercel.app", allowed) {
		t.Fatal("expected exact origin to be allowed")
	}
}

func TestOriginAllowedIgnoresTrailingSlash(t *testing.T) {
	allowed := parseAllowedOrigins("https://portofolio-ecommerce-footwear.vercel.app/")
	if !originAllowed("https://portofolio-ecommerce-footwear.vercel.app", allowed) {
		t.Fatal("expected trailing slash to be ignored")
	}
}

func TestOriginAllowedSupportsMultipleOrigins(t *testing.T) {
	allowed := parseAllowedOrigins("https://one.example.com, https://portofolio-ecommerce-footwear.vercel.app")
	if !originAllowed("https://portofolio-ecommerce-footwear.vercel.app", allowed) {
		t.Fatal("expected origin from allowlist to be allowed")
	}
}

func TestOriginAllowedSupportsLocalhost(t *testing.T) {
	allowed := parseAllowedOrigins("https://portofolio-ecommerce-footwear.vercel.app, http://localhost:3000")
	if !originAllowed("http://localhost:3000", allowed) {
		t.Fatal("expected localhost origin to be allowed")
	}
}
