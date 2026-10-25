package main

import (
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"
)

//go:embed templates/*.html static
var assets embed.FS

var pages = template.Must(template.ParseFS(assets, "templates/*.html"))

type Download struct {
	OS     string `json:"os"`
	Name   string `json:"name"`
	Detail string `json:"detail"`
	URL    string `json:"url"`
}
type Release struct {
	Version   string     `json:"version"`
	Date      string     `json:"date"`
	Notes     []string   `json:"notes"`
	Downloads []Download `json:"downloads"`
}

type Page struct {
	Release
	DownloadGroups []DownloadGroup
	Title          string
	Description    string
	Canonical      string
}

type DownloadGroup struct {
	OS, Name  string
	Downloads []Download
}

func groupDownloads(downloads []Download) []DownloadGroup {
	groups := []DownloadGroup{{OS: "windows", Name: "Windows"}, {OS: "macos", Name: "macOS"}, {OS: "linux", Name: "Linux"}}
	for i := range groups {
		for _, download := range downloads {
			if download.OS == groups[i].OS {
				groups[i].Downloads = append(groups[i].Downloads, download)
			}
		}
	}
	return groups
}

func loadRelease() (Release, error) {
	r := Release{Downloads: []Download{
		{OS: "windows", Name: "Windows installer", Detail: "Windows x64 (.exe)"},
		{OS: "windows", Name: "Windows portable", Detail: "Windows x64 (.zip)"},
		{OS: "macos", Name: "Apple Silicon", Detail: "M-series Macs (.dmg)"},
		{OS: "macos", Name: "Intel Mac", Detail: "Intel Macs (.dmg)"},
		{OS: "linux", Name: "AppImage", Detail: "Linux x64 (portable)"},
		{OS: "linux", Name: "Debian / Ubuntu", Detail: "Linux x64 (.deb)"},
		{OS: "linux", Name: "Fedora / RPM", Detail: "Linux x64 (.rpm)"},
		{OS: "linux", Name: "Arch Linux", Detail: "Linux x64 (.pkg.tar.zst)"},
	}}
	if path := os.Getenv("RELEASE_FILE"); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return r, err
		}
		err = json.Unmarshal(b, &r)
		if err != nil {
			return r, err
		}
	}
	for _, d := range r.Downloads {
		if d.URL != "" {
			if len(d.URL) < 8 || d.URL[:8] != "https://" {
				return r, &releaseError{}
			}
		}
	}
	return r, nil
}

type releaseError struct{}

func (*releaseError) Error() string { return "download URLs must use HTTPS" }
func newHandler(release Release) http.Handler {
	mux := http.NewServeMux()
	static, _ := fs.Sub(assets, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))
	render := func(name string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			page := Page{
				Release:        release,
				DownloadGroups: groupDownloads(release.Downloads),
				Title:          "Facebook Group Auto Poster for Desktop | Postika",
				Description:    "Post to multiple Facebook groups with Postika. A desktop auto poster with scheduling, multiple accounts, 33 interface languages, and clear posting logs.",
				Canonical:      "https://postika.softwrhq.com/",
			}
			if name == "downloads" {
				page.Title = "Download Postika — Facebook Group Auto Poster"
				page.Description = "Download Postika for Windows, macOS, and Linux. Choose your desktop Facebook group auto poster installer and find the latest release notes."
				page.Canonical = "https://postika.softwrhq.com/downloads"
			}
			if name == "release" {
				w.Header().Set("X-Robots-Tag", "noindex")
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if err := pages.ExecuteTemplate(w, name, page); err != nil {
				log.Printf("render %s: %v", name, err)
			}
		}
	}
	mux.HandleFunc("GET /{$}", render("home"))
	mux.HandleFunc("GET /downloads", render("downloads"))
	mux.HandleFunc("GET /partials/release", render("release"))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data: https://postika-desktop-application.b-cdn.net; font-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'")
		mux.ServeHTTP(w, r)
	})
}
func main() {
	release, err := loadRelease()
	if err != nil {
		log.Fatal(err)
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	server := &http.Server{Addr: addr, Handler: newHandler(release), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("Postika website: http://%s", addr)
	log.Fatal(server.ListenAndServe())
}
