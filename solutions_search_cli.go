package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// File-backed auth storage (no OS-specific keychain). Persist cookies to a text file.
//
// New/updated flags:
//   --login           Prompt for cookie header (visible input) and save to file
//   --logout          Delete saved cookie file
//   --auth=STR        Use this cookie header for this run
//   --save            With --auth, also persist to file
//   --store=PATH      Override auth store path (default: $XDG_CONFIG_HOME/solutions_search_cli/auth.json)
//   -u                Print only userId (same as before)
//
// Auth precedence:
//   --auth > saved file > $AUTH_HEADER_SOLUTIONS > error with hint to run --login
//
// Refresh behavior:
//   Uses http.CookieJar. Any Set-Cookie from the server updates the in-memory jar; after each
//   successful call we serialize the jar back to the store as a single Cookie header.
//   If the backend never issues Set-Cookie and the session expires, you'll need to re-login and --login again.
//
// Security note:
//   The cookie is stored as plaintext JSON with 0600 perms. Treat it like a password.

const (
	endpoint = "https://solutions.careempartner.com/user/quickSearch"
)

type quickSearchReq struct {
	ServiceProviderID int    `json:"serviceProviderId"`
	SearchKey         string `json:"searchKey"`
	Start             int    `json:"start"`
}

type quickSearchResp struct {
	Data []map[string]any `json:"data"`
}

var storePathFlag string

func main() {
	var userIDOnly bool
	var help bool
	var doLogin bool
	var doLogout bool
	var authFlag string
	var doSave bool

	flag.BoolVar(&userIDOnly, "u", false, "print only userId results")
	flag.BoolVar(&help, "h", false, "show help")
	flag.BoolVar(&doLogin, "login", false, "prompt for cookie header and save to file")
	flag.BoolVar(&doLogout, "logout", false, "delete saved cookie file")
	flag.StringVar(&authFlag, "auth", "", "cookie header to use (e.g. 'JSESSIONID=...; other=...')")
	flag.BoolVar(&doSave, "save", false, "with --auth, also persist to file")
	flag.StringVar(&storePathFlag, "store", "", "path to auth store file (default: config dir)")
	flag.Parse()

	if help {
		usage()
		return
	}

	if doLogin {
		if err := promptAndSaveCookie(); err != nil {
			fatal("login: %v", err)
		}
		fmt.Println("Saved auth cookie to:", configPath())
		return
	}
	if doLogout {
		if err := clearSavedCookie(); err != nil {
			fatal("logout: %v", err)
		}
		fmt.Println("Deleted:", configPath())
		return
	}

	args := flag.Args()
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}

	// Resolve auth header
	authHeader := strings.TrimSpace(authFlag)
	if authHeader == "" {
		if s, err := getSavedCookie(); err == nil && strings.TrimSpace(s) != "" {
			authHeader = s
		} else if env := os.Getenv("AUTH_HEADER_SOLUTIONS"); strings.TrimSpace(env) != "" {
			authHeader = env
		}
	}
	if authHeader == "" {
		fatal("No auth cookie found. Run: %s --login", os.Args[0])
	}
	if authFlag != "" && doSave {
		_ = storeCookieFile(authHeader) // best effort
	}

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Timeout: 20 * time.Second, Jar: jar}

	endpointURL, _ := url.Parse(endpoint)
	seedJarWithCookieHeader(jar, endpointURL, authHeader)

	for _, arg := range args {
		term := normalizeTerm(arg)
		updated, err := queryAndPrint(client, endpointURL, term, userIDOnly)
		if err != nil {
			var he *httpErr
			if errors.As(err, &he) && (he.status == 401 || he.status == 403) {
				fmt.Fprintf(os.Stderr, "auth failed (%d). Your session may be expired. Re-run with --login.", he.status)
			} else {
				fmt.Fprintf(os.Stderr, "error: %v", err)
			}
		}
		if updated {
			if hdr := buildCookieHeaderFromJar(jar, endpointURL); hdr != "" {
				_ = storeCookieFile(hdr)
			}
		}
	}
}

func usage() {
	fmt.Println(`Usage: solutions_search_cli [flags] <term> [<term> ...]

Flags:
  -u             Print only userId (like filter-solutions-userid.jq)
  -h             Show help
  --login        Prompt once for cookie header and save to file
  --logout       Delete saved cookie file
  --auth=STR     Use this cookie header for this run
  --save         With --auth, also persist to file
  --store=PATH   Override auth store path (default below)

Auth resolution order:
  --auth > saved file > $AUTH_HEADER_SOLUTIONS > error

Default store path:
  ` + defaultPathForHelp() + `
`) }

func normalizeTerm(s string) string {
	if strings.HasPrefix(s, "+") {
		s = strings.TrimPrefix(s, "+")
	}
	if len(s) < 9 {
		s = "#" + s
	}
	return s
}

type httpErr struct{ status int; body string }

func (e *httpErr) Error() string { return fmt.Sprintf("HTTP %d: %s", e.status, e.body) }

// Returns (updated, error).
func queryAndPrint(client *http.Client, endpointURL *url.URL, term string, userIDOnly bool) (bool, error) {
	payload := quickSearchReq{ServiceProviderID: 1, SearchKey: term, Start: 0}
	b, _ := json.Marshal(payload)

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(b))
	if err != nil { return false, err }
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil { return false, err }
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return false, &httpErr{status: resp.StatusCode, body: strings.TrimSpace(string(body))}
	}

	var out quickSearchResp
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&out); err != nil { return false, fmt.Errorf("decode: %w", err) }

	for _, row := range out.Data {
		if userIDOnly {
			fmt.Println(stringify(row["userId"]))
			continue
		}
		phone := padRight(stringify(row["phoneNumber"]), 14)
		uid := padRight(stringify(row["userId"]), 10)
		name := padRight(stringify(row["fullName"]), 24)
		country := padRight(orDash(row["countryId"]), 4)
		companyID := padRight(orDash(row["companyId"]), 6)
		email := padRight(stringify(row["email"]), 54)
		company := padRight(orDash(row["companyName"]), 32)

		fmt.Print(phone, uid, name, country, companyID, email, company, "\n")
	}

	return true, nil
}

// ========================= Cookie storage (file) =========================

func configPath() string {
	if storePathFlag != "" { return storePathFlag }
	dir, _ := os.UserConfigDir()
	if dir == "" { dir = os.Getenv("HOME") }
	if dir == "" { dir = "." }
	return filepath.Join(dir, "solutions_search_cli", "auth.json")
}

func defaultPathForHelp() string {
	dir, _ := os.UserConfigDir()
	if dir == "" { dir = "$HOME" }
	return filepath.Join(dir, "solutions_search_cli", "auth.json")
}

func promptAndSaveCookie() error {
	fmt.Print("Paste cookie header (visible input, e.g., JSESSIONID=...; other=...): ")
	reader := bufio.NewReader(os.Stdin)
	s, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) { return err }
	s = strings.TrimSpace(s)
	if s == "" { return errors.New("empty cookie") }
	return storeCookieFile(s)
}

func storeCookieFile(s string) error {
	p := configPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil { return err }
	f, err := os.OpenFile(p, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil { return err }
	defer f.Close()
	enc := json.NewEncoder(f)
	return enc.Encode(map[string]string{"cookie": s})
}

func getSavedCookie() (string, error) {
	p := configPath()
	f, err := os.Open(p)
	if err != nil { return "", err }
	defer f.Close()
	dec := json.NewDecoder(bufio.NewReader(f))
	var m map[string]string
	if err := dec.Decode(&m); err != nil { return "", err }
	return strings.TrimSpace(m["cookie"]), nil
}

func clearSavedCookie() error {
	return os.Remove(configPath())
}

// ========================= utils =========================

func seedJarWithCookieHeader(jar http.CookieJar, u *url.URL, header string) {
	pairs := strings.Split(header, ";")
	var cookies []*http.Cookie
	for _, p := range pairs {
		p = strings.TrimSpace(p)
		if p == "" { continue }
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 { continue }
		name := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])
		if name == "" { continue }
		cookies = append(cookies, &http.Cookie{Name: name, Value: value, Path: "/"})
	}
	if len(cookies) > 0 {
		jar.SetCookies(u, cookies)
	}
}

func buildCookieHeaderFromJar(jar http.CookieJar, u *url.URL) string {
	cookies := jar.Cookies(u)
	if len(cookies) == 0 { return "" }
	parts := make([]string, 0, len(cookies))
	for _, c := range cookies {
		if c.Name == "" { continue }
		parts = append(parts, fmt.Sprintf("%s=%s", c.Name, c.Value))
	}
	return strings.Join(parts, "; ")
}

func stringify(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case json.Number:
		return t.String()
	case float64:
		if t == float64(int64(t)) { return fmt.Sprintf("%d", int64(t)) }
		return fmt.Sprintf("%v", t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

func orDash(v any) string {
	if v == nil { return "-" }
	s := strings.TrimSpace(stringify(v))
	if s == "" { return "-" }
	return s
}

func padRight(s string, width int) string {
	if width <= 0 { return s }
	if len(s) >= width { return s }
	return s + strings.Repeat(" ", width-len(s))
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"", args...)
	os.Exit(1)
}
