package main

import (
	"context"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// A native checkbox toggles before React handles the event. Facebook's
// aria-checked reflects the acknowledged switch state, including controlled
// inputs whose native checked property temporarily disagrees with the UI.
const marketplaceHomeSwitchState = `function(){
 const aria=this.getAttribute('aria-checked');
 if(aria===null)return this.checked===true;
 if(aria!=='true' && aria!=='false')throw new Error('Unsupported Facebook switch state');
 return aria==='true';
}`

func marketplaceHomeSwitch(ctx context.Context, page *rod.Page, name string, on bool) error {
	verify, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	var stableSince, retryAt time.Time
	attempts := 0
	for {
		// React can replace the input after either a mouse or keyboard event.
		field, err := marketplaceFieldWith(verify, page, name, `input[type="checkbox"][role="switch"]`, marketplaceHomeControl)
		if err != nil {
			return fmt.Errorf("Could not confirm Facebook %s: %w", name, err)
		}
		state, err := field.Eval(marketplaceHomeSwitchState)
		if err != nil {
			_ = field.Release()
			return fmt.Errorf("Could not inspect Facebook %s: %w", name, err)
		}
		if state.Value.Bool() == on {
			if stableSince.IsZero() {
				stableSince = time.Now()
			}
			_ = field.Release()
			if time.Since(stableSince) >= 450*time.Millisecond {
				return nil
			}
		} else {
			stableSince = time.Time{}
			if attempts < 2 && !time.Now().Before(retryAt) {
				if attempts == 0 {
					err = field.Click(proto.InputMouseButtonLeft, 1)
				} else {
					// Retry one unacknowledged mouse click through keyboard activation.
					err = field.Type(input.Space)
				}
				attempts++
				retryAt = time.Now().Add(time.Second)
			}
			_ = field.Release()
			if err != nil {
				return fmt.Errorf("Could not set Facebook %s: %w", name, err)
			}
		}
		select {
		case <-verify.Done():
			return fmt.Errorf("Facebook did not confirm %s was set to %t: %w", name, on, verify.Err())
		case <-ticker.C:
		}
	}
}

const marketplaceHomeDateFormats = `(iso)=>{
 const field=(` + marketplaceHomeControl + `)('Date available','input[role="combobox"]');
 const locale=field?.closest('[lang]')?.lang || document.documentElement.lang || navigator.language;
 const date=new Date(iso+'T12:00:00');
 return [iso,...['2-digit','short','long'].map(month=>new Intl.DateTimeFormat(locale.replaceAll('_','-'),{
  calendar:'gregory',numberingSystem:'latn',year:'numeric',month,day:month==='2-digit'?'2-digit':'numeric'
 }).format(date))];
}`

const marketplaceHomeDateConfirmed = `(iso)=>{
 const field=(` + marketplaceHomeControl + `)('Date available','input[role="combobox"]');
 if(!field || field.getAttribute('aria-invalid')==='true' || !field.validity.valid)return false;
 // Facebook reformats numeric entry to a localized month name after blur.
 // Compare complete date tokens, preserving their order and the month name;
 // dropping non-numeric text would lose the month and accept wrong dates.
 const tokens=value=>(value.normalize('NFKC').replace(/[\u0660-\u0669\u06f0-\u06f9]/g,d=>String(d.charCodeAt(0)%16))
  .toLocaleLowerCase().match(/[\p{L}\p{N}]+/gu)||[]).map(token=>/^[0-9]+$/.test(token)?String(Number(token)):token).join('|');
 const actual=tokens(field.value);
 return (` + marketplaceHomeDateFormats + `)(iso).some(display=>tokens(display)===actual);
}`

func marketplaceHomeDate(ctx context.Context, page *rod.Page, iso string) error {
	field, err := marketplaceFieldWith(ctx, page, "Date available", `input[role="combobox"]`, marketplaceHomeControl)
	if err != nil {
		return err
	}
	display, err := page.Context(ctx).Eval(`(iso)=>(`+marketplaceHomeDateFormats+`)(iso)[1]`, iso)
	if err != nil {
		return fmt.Errorf("Could not format Date available for Facebook: %w", err)
	}
	if err := field.SelectAllText(); err != nil {
		return err
	}
	if err := field.Input(display.Value.Str()); err != nil {
		return fmt.Errorf("Could not fill Date available: %w", err)
	}
	// Commit the editable date through the browser's normal blur handling.
	if err := field.Type(input.Tab); err != nil {
		return err
	}
	verify, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := page.Context(verify).Wait(rod.Eval(marketplaceHomeDateConfirmed, iso)); err != nil {
		return fmt.Errorf("Facebook did not confirm Date available: %w", err)
	}
	return nil
}
