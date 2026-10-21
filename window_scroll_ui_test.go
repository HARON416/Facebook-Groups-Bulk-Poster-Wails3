package main

import (
	"testing"

	"github.com/go-rod/rod"
)

// Exercise real wheel input, including a document overflow fixture. Reaching
// either end of a pane must not scroll its native window's frame or title bar.
func assertWindowScrollContained(t *testing.T, page *rod.Page, paneSelector, titlebarSelector string) {
	t.Helper()
	page.MustActivate()
	page.MustEval(`() => {const fixture=document.createElement('div');fixture.id='document-overflow-fixture';fixture.style.height='1600px';document.body.append(fixture);}`)
	defer page.MustEval(`() => {document.querySelector('#document-overflow-fixture')?.remove();window.scrollTo(0,0);}`)
	geometry := page.MustEval(`(paneSelector,titlebarSelector) => {
		const pane=document.querySelector(paneSelector),r=pane.getBoundingClientRect();
		pane.scrollTop=0;
		window.scrollBoundaryTitleTop=document.querySelector(titlebarSelector).getBoundingClientRect().top;
		return {x:r.left+8,y:r.top+Math.min(40,r.height/2),overflow:pane.scrollHeight>pane.clientHeight};
	}`, paneSelector, titlebarSelector)
	if !geometry.Get("overflow").Bool() {
		t.Fatalf("scroll fixture has no overflowing pane: %s", paneSelector)
	}
	page.Mouse.MustMoveTo(geometry.Get("x").Num(), geometry.Get("y").Num()).MustScroll(0, 180)
	if !page.MustEval(`async selector => {await new Promise(r=>setTimeout(r,150));return document.querySelector(selector).scrollTop>0;}`, paneSelector).Bool() {
		t.Fatalf("content pane stopped scrolling: %s", paneSelector)
	}
	for _, bottom := range []bool{true, false} {
		page.MustEval(`(selector,bottom) => {const pane=document.querySelector(selector);pane.scrollTop=bottom?pane.scrollHeight:0;}`, paneSelector, bottom)
		delta := -1200.0
		if bottom {
			delta = 1200
		}
		page.Mouse.MustScroll(0, delta).MustScroll(0, delta)
		if !page.MustEval(`async (paneSelector,titlebarSelector,bottom) => {
			await new Promise(r=>setTimeout(r,150));
			const pane=document.querySelector(paneSelector),frame=document.querySelector('.auxiliary-window,.workspace').getBoundingClientRect();
			return window.scrollY===0 && Math.abs(frame.top)<1 && Math.abs(frame.bottom-innerHeight)<1
				&& Math.abs(document.querySelector(titlebarSelector).getBoundingClientRect().top-window.scrollBoundaryTitleTop)<1
				&& (bottom?Math.abs(pane.scrollHeight-pane.clientHeight-pane.scrollTop)<2:pane.scrollTop===0);
		}`, paneSelector, titlebarSelector, bottom).Bool() {
			t.Fatalf("wheel scrolling escaped %s at bottom=%v", paneSelector, bottom)
		}
	}
	page.MustEval(`selector => {document.querySelector(selector).scrollTop=0;}`, paneSelector)
}

func assertTextareaScrollContained(t *testing.T, page *rod.Page) {
	t.Helper()
	page.MustEval(`() => {const input=document.querySelector('textarea');input.value='Long rewrite instruction\n'.repeat(100);input.scrollIntoView({block:'center',behavior:'instant'});input.scrollTop=input.scrollHeight;window.textareaParentTop=document.querySelector('.auxiliary-content').scrollTop;}`)
	point := page.MustEval(`() => {const r=document.querySelector('textarea').getBoundingClientRect();return {x:r.left+r.width/2,y:r.top+r.height/2};}`)
	page.Mouse.MustMoveTo(point.Get("x").Num(), point.Get("y").Num()).MustScroll(0, 1200)
	if !page.MustEval(`async () => {await new Promise(r=>setTimeout(r,150));return document.querySelector('.auxiliary-content').scrollTop===window.textareaParentTop && window.scrollY===0 && document.querySelector('.auxiliary-title-bar').getBoundingClientRect().top===2;}`).Bool() {
		snapshot := page.MustEval(`() => {const input=document.querySelector('textarea'),r=input.getBoundingClientRect();return {parentBefore:window.textareaParentTop,parentAfter:document.querySelector('.auxiliary-content').scrollTop,windowTop:window.scrollY,titleTop:document.querySelector('.auxiliary-title-bar').getBoundingClientRect().top,textarea:{top:r.top,bottom:r.bottom,scrollTop:input.scrollTop,scrollHeight:input.scrollHeight,clientHeight:input.clientHeight,overscroll:getComputedStyle(input).overscrollBehavior},hit:document.elementFromPoint(r.left+r.width/2,r.top+r.height/2)?.tagName};}`)
		t.Fatalf("scrolling past the textarea moved its settings pane or title bar: %v", snapshot)
	}
}
