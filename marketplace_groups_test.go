package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
)

func TestMarketplaceGroupsBrowserIntegration(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, ok := launcher.LookPath()
	if !ok {
		t.Fatal("Chrome required")
	}
	capture, err := os.ReadFile("testdata/marketplace/groups.html")
	if err != nil {
		t.Fatal(err)
	}
	harness := `<script>
window.clicks=[];window.unchecked=0;window.published=0;
const template=document.querySelector('[role=form] [role=checkbox]').cloneNode(true);
window.setup=(count,disabled=0,selected=0)=>{
 const form=document.querySelector('[role=form]');form.innerHTML='';form.removeAttribute('aria-busy');clicks=[];unchecked=0;
 for(let i=0;i<count;i++){
  const row=template.cloneNode(true);row.dataset.index=i;row.querySelector('span[dir=auto]').textContent='مجموعة '+i;
  row.setAttribute('aria-checked',i>=count-selected?'true':'false');row.removeAttribute('aria-disabled');if(i<disabled)row.setAttribute('aria-disabled','true');form.append(row);
 }
};
document.addEventListener('click',event=>{
 const row=event.target.closest('[role=form] [role=checkbox]');if(!row||row.closest('[aria-disabled=true]'))return;
 clicks.push(Number(row.dataset.index));if(row.getAttribute('aria-checked')==='true')unchecked++;
 row.setAttribute('aria-checked',row.getAttribute('aria-checked')==='true'?'false':'true');
 const form=row.closest('[role=form]');form.replaceWith(form.cloneNode(true));
});
document.querySelectorAll('[role=button]').forEach(el=>el.onclick=()=>published++);
window.summary=()=>({checked:[...document.querySelectorAll('[role=form] [role=checkbox][aria-checked=true]')].length,clicks,unchecked,published});
setup(26,1,2);
</script>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<body>`, string(capture), harness)
	}))
	defer server.Close()
	service := &GroupsService{profileDir: t.TempDir(), chromePath: chrome, headless: true}
	t.Cleanup(func() { _ = service.shutdown(true) })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	page, err := service.prepareMarketplaceTestPage(ctx, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := page.Context(ctx).WaitLoad(); err != nil {
		t.Fatal(err)
	}
	eval := func(js string) {
		t.Helper()
		if _, err := page.Context(ctx).Eval(js); err != nil {
			t.Fatal(err)
		}
	}
	check := func(wantSelected, wantClicks int) {
		t.Helper()
		result, err := page.Context(ctx).Eval(`()=>summary()`)
		if err != nil {
			t.Fatal(err)
		}
		v := result.Value
		if v.Get("checked").Int() != wantSelected || len(v.Get("clicks").Arr()) != wantClicks || v.Get("unchecked").Int() != 0 || v.Get("published").Int() != 0 {
			t.Fatalf("Incorrect group selection: %v", v)
		}
	}
	t.Run("limit-includes-preselected-and-skips-disabled", func(t *testing.T) {
		if err := selectMarketplaceGroups(ctx, page); err != nil {
			t.Fatal(err)
		}
		check(20, 18)
		result, err := page.Context(ctx).Eval(`()=>clicks.some(index=>index===0||index===24||index===25)`)
		if err != nil || result.Value.Bool() {
			t.Fatalf("Disabled/preselected row clicked: %v %v", result, err)
		}
		if err := selectMarketplaceGroups(ctx, page); err != nil {
			t.Fatal(err)
		}
		check(20, 18)
	})
	t.Run("fewer-than-twenty", func(t *testing.T) {
		eval(`()=>setup(10,2)`)
		if err := selectMarketplaceGroups(ctx, page); err != nil {
			t.Fatal(err)
		}
		check(8, 8)
	})
	t.Run("empty", func(t *testing.T) {
		eval(`()=>setup(0)`)
		if err := selectMarketplaceGroups(ctx, page); err != nil {
			t.Fatal(err)
		}
		check(0, 0)
	})
	t.Run("delayed-rows", func(t *testing.T) {
		eval(`()=>{setup(0);document.querySelector('[role=form]').setAttribute('aria-busy','true');setTimeout(()=>setup(5),200)}`)
		if err := selectMarketplaceGroups(ctx, page); err != nil {
			t.Fatal(err)
		}
		check(5, 5)
	})
	t.Run("lazy-loaded-rows", func(t *testing.T) {
		eval(`()=>{setup(4);const form=document.querySelector('[role=form]');form.style.height='120px';form.style.overflowY='auto';form.onscroll=()=>{form.onscroll=null;setTimeout(()=>{const current=document.querySelector('[role=form]');for(let i=4;i<25;i++){const row=template.cloneNode(true);row.dataset.index=i;row.querySelector('span[dir=auto]').textContent='مجموعة '+i;row.setAttribute('aria-checked','false');current.append(row)}},100)};}`)
		// Replacements preserve the scroll container but remove property handlers;
		// attach this lazy fixture's listener to the stable document instead.
		eval(`()=>{document.querySelector('[role=form]').onscroll=null;window.lazyAdded=false;document.addEventListener('scroll',event=>{if(lazyAdded||event.target.getAttribute?.('role')!=='form')return;lazyAdded=true;setTimeout(()=>{const form=document.querySelector('[role=form]');for(let i=4;i<25;i++){const row=template.cloneNode(true);row.dataset.index=i;row.querySelector('span[dir=auto]').textContent='مجموعة '+i;row.setAttribute('aria-checked','false');form.append(row)}},100)},true)}`)
		if err := selectMarketplaceGroups(ctx, page); err != nil {
			t.Fatal(err)
		}
		check(20, 20)
	})
	t.Run("preexisting-over-limit", func(t *testing.T) {
		eval(`()=>setup(21,0,21)`)
		if err := selectMarketplaceGroups(ctx, page); err == nil {
			t.Fatal("Over-limit selection accepted")
		}
		check(21, 0)
	})
	t.Run("cancelled", func(t *testing.T) {
		eval(`()=>setup(5)`)
		stopped, stop := context.WithCancel(ctx)
		stop()
		if err := selectMarketplaceGroups(stopped, page); !errors.Is(err, context.Canceled) {
			t.Fatalf("Cancellation ignored: %v", err)
		}
		check(0, 0)
	})
}
