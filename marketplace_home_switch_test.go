package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
)

func TestMarketplaceHomeSwitchBrowserIntegration(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, ok := launcher.LookPath()
	if !ok {
		t.Fatal("Chrome required")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<body><main><input type="file" accept="image/*">
<input type="checkbox" role="switch" aria-checked="false">
<input id="cat" type="checkbox" role="switch" aria-checked="false">
<input type="checkbox" role="switch" aria-checked="false">
<input type="checkbox" role="switch" aria-checked="false">
</main><script>
window.clicks=0;window.acknowledged=false;const mode=location.pathname.slice(1);const cat=document.querySelector('#cat');
if(mode==='already-on'||mode==='off'){cat.setAttribute('aria-checked','true');cat.checked=false}
if(mode==='stale-native')cat.checked=true;
document.addEventListener('click',event=>{
 const field=event.target;if(field.id!=='cat')return;clicks++;
 if(mode==='never-ack')return;
 if(mode==='ignored-click'&&clicks===1){setTimeout(()=>field.checked=false,20);return}
 const on=field.getAttribute('aria-checked')!=='true';
 setTimeout(()=>{field.checked=on;field.setAttribute('aria-checked',String(on));field.replaceWith(field.cloneNode(true));acknowledged=true},250);
});
</script></body>`)
	}))
	defer server.Close()
	service := &GroupsService{profileDir: t.TempDir(), chromePath: chrome, headless: true}
	t.Cleanup(func() { _ = service.shutdown(true) })
	for _, mode := range []string{"delayed", "ignored-click", "stale-native", "already-on", "off", "never-ack"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			page, err := service.prepareMarketplaceTestPage(ctx, server.URL+"/"+mode)
			if err != nil {
				t.Fatal(err)
			}
			if err := page.Context(ctx).WaitLoad(); err != nil {
				t.Fatal(err)
			}
			on := mode != "off"
			operation := ctx
			if mode == "never-ack" {
				var stop context.CancelFunc
				operation, stop = context.WithTimeout(ctx, 3*time.Second)
				defer stop()
			}
			err = marketplaceHomeSwitch(operation, page, "Cat friendly", on)
			if mode == "never-ack" {
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("Unacknowledged native toggles must fail, got %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			result, err := page.Context(ctx).Eval(`()=>({clicks,acknowledged,on:document.querySelector('#cat').getAttribute('aria-checked')==='true'})`)
			if err != nil {
				t.Fatal(err)
			}
			wantClicks := 1
			if mode == "ignored-click" || mode == "never-ack" {
				wantClicks = 2
			} else if mode == "already-on" {
				wantClicks = 0
			}
			if result.Value.Get("clicks").Int() != wantClicks {
				t.Fatalf("Unexpected switch activations: %v", result.Value)
			}
			if mode != "never-ack" && result.Value.Get("on").Bool() != on {
				t.Fatalf("Wrong acknowledged state: %v", result.Value)
			}
			if mode != "never-ack" && mode != "already-on" && !result.Value.Get("acknowledged").Bool() {
				t.Fatal("Returned before the app acknowledged the toggle")
			}
		})
	}
}
