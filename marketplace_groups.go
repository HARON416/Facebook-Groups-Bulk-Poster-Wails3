package main

import (
	"context"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// These are group rows in the supplied destination form, scoped through the
// second-step navigation. Accessible names and generated classes are not matched.
const marketplaceGroupRows = `() => {
 const navigation=(` + marketplaceStructuralNavigation + `)();
 if(!navigation || navigation.step!==2)return null;
 const visible=el=>el.getClientRects().length && !el.closest('[hidden],[aria-hidden="true"]') && getComputedStyle(el).visibility!=='hidden';
 const forms=[...navigation.main.querySelectorAll('[role="form"]')].filter(visible);
 if(forms.length>1)throw new Error('Multiple Marketplace destination forms');
 if(!forms.length)return [];
 const rows=[...forms[0].querySelectorAll('[role="checkbox"][aria-checked]')].filter(visible);
 return rows.map(el=>{
  const name=el.querySelector('span[dir="auto"]')?.textContent.trim();
  if(!name || !el.querySelector('svg image,img'))throw new Error('Unsupported Marketplace group row layout');
  const state=el.getAttribute('aria-checked');
  if(state!=='true' && state!=='false')throw new Error('Unsupported Marketplace group checkbox state');
  return {el,name,checked:state==='true',disabled:!!el.closest('[aria-disabled="true"],[inert],[disabled]')||el.matches(':disabled')};
 });
}`

const marketplaceGroupSnapshot = `()=>{
 const rows=(` + marketplaceGroupRows + `)();
 if(!rows)throw new Error('Marketplace group selection requires step two');
 const navigation=(` + marketplaceStructuralNavigation + `)();
 const loading=[...navigation.main.querySelectorAll('[role="form"] [data-visualcompletion="loading-state"],[role="form"][aria-busy="true"]')].some(el=>el.getClientRects().length&&!el.closest('[hidden],[aria-hidden="true"]'));
 return {checked:rows.filter(row=>row.checked).length,available:rows.filter(row=>!row.checked&&!row.disabled).length,total:rows.length,loading};
}`

const marketplaceNextGroup = `()=>{
 const rows=(` + marketplaceGroupRows + `)();
 if(!rows || rows.filter(row=>row.checked).length>=20)return null;
 return rows.find(row=>!row.checked&&!row.disabled)?.el||null;
}`

// Return the scroll position after revealing the last currently rendered row.
// Facebook can append more groups as the sidebar reaches the bottom.
const marketplaceRevealMoreGroups = `()=>{
 const rows=(` + marketplaceGroupRows + `)();
 const last=rows?.at(-1)?.el;if(!last)return false;
 let container=last.parentElement;
 while(container && container!==document.body) {
  if(container.scrollHeight>container.clientHeight+4 && /auto|scroll/.test(getComputedStyle(container).overflowY)) {
   const before=container.scrollTop;container.scrollTop=container.scrollHeight;
   return container.scrollTop!==before;
  }
  container=container.parentElement;
 }
 last.scrollIntoView({block:'nearest'});
 return false;
}`

func selectMarketplaceGroups(ctx context.Context, page *rod.Page) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	stable := 0
	lastTotal := -1
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("Could not finish Marketplace group selection: %w", err)
		}
		snapshot, err := page.Context(ctx).Eval(marketplaceGroupSnapshot)
		if err != nil {
			return fmt.Errorf("Could not inspect Marketplace groups: %w", err)
		}
		checked := snapshot.Value.Get("checked").Int()
		total := snapshot.Value.Get("total").Int()
		if total != lastTotal {
			stable = 0
			lastTotal = total
		}
		if checked > 20 {
			return fmt.Errorf("Facebook already has more than 20 groups selected; review the selection in Chrome")
		}
		if checked == 20 {
			return nil
		}
		if snapshot.Value.Get("available").Int() == 0 {
			if _, err := page.Context(ctx).Eval(marketplaceRevealMoreGroups); err != nil {
				return err
			}
			// Allow appended rows to render. Recheck twice after reaching the end,
			// instead of repeatedly scrolling or holding an old list of controls.
			stable++
			if snapshot.Value.Get("loading").Bool() {
				stable = 0
			}
			if stable >= 3 {
				return nil
			}
			timer := time.NewTimer(350 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return fmt.Errorf("Could not finish Marketplace group selection: %w", ctx.Err())
			case <-timer.C:
			}
			continue
		}
		stable = 0
		// Resolve and recheck the next unchecked control each time: clicks may
		// replace all rows. Already selected groups are never toggled off.
		row, err := page.Context(ctx).ElementByJS(rod.Eval(marketplaceNextGroup))
		if err != nil {
			return fmt.Errorf("Could not find an available Marketplace group: %w", err)
		}
		if err := row.Click(proto.InputMouseButtonLeft, 1); err != nil {
			return fmt.Errorf("Could not select a Marketplace group: %w", err)
		}
		verify, stop := context.WithTimeout(ctx, 5*time.Second)
		err = page.Context(verify).Wait(rod.Eval(`(previous)=>{const rows=(`+marketplaceGroupRows+`)();return !!rows && rows.filter(row=>row.checked).length>previous}`, checked))
		stop()
		if err != nil {
			return fmt.Errorf("Facebook did not confirm the group selection; review it in Chrome: %w", err)
		}
	}
}
