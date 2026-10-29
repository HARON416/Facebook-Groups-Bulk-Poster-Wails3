package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
)

// The supplied structure with French text and an asynchronous expansion.
const marketplaceDetailsFixture = `<div id="fixture-details" style="position:relative;width:320px;padding:12px" role="button" tabindex="0" aria-expanded="false" onclick="window.detailsClicks=(window.detailsClicks||0)+1; const control=this; setTimeout(()=>{control.setAttribute('aria-expanded','true'); document.querySelector('#fixture-location').hidden=false},700)"><div><div><div><span id="fixture-details-label">Plus de détails</span></div><div><span>Ajoutez des détails.</span></div></div><i data-visualcompletion="css-img"></i></div><div role="none" data-visualcompletion="ignore" style="position:absolute;inset:0"></div></div>`
const marketplaceDetailsPageFixture = `<body class="UIPage_LoggedIn"><aside role="complementary">` + marketplaceDetailsFixture + `<input id="fixture-location" hidden></aside></body>`

func TestMarketplaceDetailsExpansionIntegration(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, found := launcher.LookPath()
	if !found {
		t.Fatal("Chrome required")
	}
	// A matching control outside the composer must not be clicked.
	outside := strings.ReplaceAll(marketplaceDetailsFixture, "fixture-details", "outside-details")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<body class="UIPage_LoggedIn"><header>` + outside + `</header><aside role="complementary" id="fixture-sidebar"></aside><script>window.detailsClicks=0</script></body>`))
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s := &GroupsService{profileDir: t.TempDir(), chromePath: chrome, headless: true}
	t.Cleanup(func() { _ = s.ServiceShutdown(); time.Sleep(200 * time.Millisecond) })
	target := server.URL + "/marketplace/create/item"
	waitState := func(want string) {
		t.Helper()
		deadline := time.Now().Add(6 * time.Second)
		for time.Now().Before(deadline) {
			if s.marketplaceLocationLoad.snapshot().State == want {
				return
			}
			time.Sleep(25 * time.Millisecond)
		}
		t.Fatalf("status=%+v; want %s", s.marketplaceLocationLoad.snapshot(), want)
	}
	open := func() {
		t.Helper()
		if err := s.openMarketplaceLocation(ctx, target); err != nil {
			t.Fatal(err)
		}
		waitState("expanding_details")
	}
	checkClicks := func(want int) {
		t.Helper()
		result, err := s.marketplaceLocationPage.Context(ctx).Eval(`() => window.detailsClicks`)
		if err != nil || result.Value.Int() != want {
			t.Fatalf("clicks=%v; want %d; error=%v", result, want, err)
		}
	}
	open()
	page := s.marketplaceLocationPage.Context(ctx)
	checkClicks(0)
	// Poll through late rendering, hidden controls, and disabled controls.
	if _, err := page.Eval(`html => {document.querySelector('#fixture-sidebar').innerHTML=html+'<input id="fixture-location" hidden>';document.querySelector('#fixture-details').hidden=true;}`, marketplaceDetailsFixture); err != nil {
		t.Fatal(err)
	}
	time.Sleep(600 * time.Millisecond)
	checkClicks(0)
	if _, err := page.Eval(`() => {const control=document.querySelector('#fixture-details');control.hidden=false;control.setAttribute('aria-disabled','true');}`); err != nil {
		t.Fatal(err)
	}
	time.Sleep(600 * time.Millisecond)
	checkClicks(0)
	if _, err := page.Eval(`() => {const control=document.querySelector('#fixture-details');control.removeAttribute('aria-disabled');control.style.marginTop='1200px';}`); err != nil {
		t.Fatal(err)
	}
	waitState("ready")
	checkClicks(1)
	result, err := page.Eval(`() => document.querySelector('#fixture-details').getAttribute('aria-expanded')==='true' && !document.querySelector('#fixture-location').hidden`)
	if err != nil || !result.Value.Bool() {
		t.Fatal("details were not expanded to reveal location")
	}
	// A click accepted by Chrome before React attaches its handler must retry.
	open()
	if _, err := page.Eval(`html => {document.querySelector('#fixture-sidebar').innerHTML=html+'<input id="fixture-location" hidden>';const control=document.querySelector('#fixture-details');window.fixtureHandler=control.getAttribute('onclick');control.removeAttribute('onclick');control.addEventListener('click',()=>window.detailsClicks++);}`, marketplaceDetailsFixture); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		result, err := page.Eval(`() => window.detailsClicks`)
		if err == nil && result.Value.Int() > 0 {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	checkClicks(1)
	if _, err := page.Eval(`() => {const control=document.querySelector('#fixture-details');control.setAttribute('onclick',window.fixtureHandler);control.addEventListener('click',()=>window.detailsClicks--);}`); err != nil {
		t.Fatal(err)
	}
	waitState("ready")
	checkClicks(2)
	// A complementary region elsewhere does not imply the composer lives there.
	open()
	if _, err := page.Eval(`html => {const main=document.createElement('main');main.innerHTML=html+'<input id="fixture-location" hidden>';document.body.append(main);}`, marketplaceDetailsFixture); err != nil {
		t.Fatal(err)
	}
	waitState("ready")
	checkClicks(1)
	// Already expanded controls are acknowledged without a second click.
	open()
	if _, err := page.Eval(`html => {document.querySelector('#fixture-sidebar').innerHTML=html;document.querySelector('#fixture-details').setAttribute('aria-expanded','true');}`, marketplaceDetailsFixture); err != nil {
		t.Fatal(err)
	}
	waitState("ready")
	checkClicks(0)
	// Ambiguous controls must never cause a click on the wrong field.
	open()
	if _, err := page.Eval(`html => document.querySelector('#fixture-sidebar').innerHTML=html+html`, marketplaceDetailsFixture); err != nil {
		t.Fatal(err)
	}
	waitState("details_error")
	checkClicks(0)
	// A missing accordion remains cancellable without closing the browser.
	open()
	// A loaded page with no matching control yields a finite, retryable failure.
	expired := marketplaceDetailsAttempt{deadline: time.Now().Add(-time.Second)}
	if _, err := expandMarketplaceDetails(ctx, page, &expired); !errors.Is(err, errMarketplaceDetails) {
		t.Fatalf("missing accordion did not report a preparation error: %v", err)
	}
	load := s.marketplaceLocationLoad
	load.cancel()
	waitState("cancelled")
	if _, err := page.Info(); err != nil {
		t.Fatal("cancelled preparation closed the browser")
	}
}
