package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-rod/rod"
)

type PostUpload struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

const postDialogSelector = `div[role="dialog"][aria-modal="true"]`
const postImagesSelector = `input.x1s85apg[type="file"][multiple][accept*="image/"]`
const postBodySelector = `div[contenteditable="true"][role="textbox"][data-lexical-editor="true"]`
const postSubmitSelector = `[role="button"], button`

// Match the extension's composer form/footer structure. Facebook changes the
// action's classes and can omit aria-disabled when the action becomes enabled.
func waitPostSubmit(ctx context.Context, page *rod.Page, enabled bool) (*rod.Element, error) {
	// Check the action itself; composer animations and progress indicators can
	// continue after Facebook has enabled submission.
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		dialog, err := waitPostElement(ctx, page, nil, postDialogSelector, true)
		if err != nil {
			return nil, err
		}
		buttons, err := dialog.Elements(postSubmitSelector)
		if err == nil {
			for _, button := range buttons {
				match, err := button.Eval(`function(enabled) {
                    const dialog = this.closest('[role="dialog"][aria-modal="true"]');
                    const visible = el => el.isConnected && el.getClientRects().length &&
                        getComputedStyle(el).visibility !== 'hidden';
                    const editors = [...dialog.querySelectorAll('[role="textbox"][contenteditable="true"]')].filter(visible);
                    if (editors.length !== 1) return false;
                    const form = editors[0].closest('form');
                    if (!form || !dialog.contains(form) || form.method.toLowerCase() !== 'post' ||
                        !form.querySelector('input[type="submit"], button[type="submit"]')) return false;
                    const actions = [...form.querySelectorAll('[role="button"], button')].filter(visible);
                    return actions.at(-1) === this && !!this.textContent.trim() &&
                        !!this.querySelector('span') && !this.querySelector('svg, img, i, input') &&
                        !!this.querySelector('[role="none"][data-visualcompletion="ignore"]') &&
                        this.getBoundingClientRect().width >= form.getBoundingClientRect().width * 0.8 &&
                        (!enabled || (this.getAttribute('aria-disabled') !== 'true' && !this.hasAttribute('disabled')));
                }`, enabled)
				if err == nil && match.Value.Bool() {
					return button, nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func fillGroupComposer(ctx context.Context, page *rod.Page, body string, images []PostUpload, retainedDir *string, progress ...func(string)) error {
	report := func(message string) {
		for _, update := range progress {
			update(message)
		}
	}

	report("Waiting for the composer controls and layout…")
	dialog, err := waitReadyPostDialog(ctx, page, len(images) > 0)
	if err != nil {
		return err
	}

	if len(images) > 0 {
		if err := postingCheckpoint(ctx); err != nil {
			return err
		}
		report("Attaching images…")
		input, err := waitPostElement(ctx, page, dialog, postImagesSelector, false)
		if err != nil {
			return fmt.Errorf("find composer image input: %w", err)
		}
		dir, paths, err := stagePostImages(images)
		if err != nil {
			return err
		}
		if err := input.SetFiles(paths); err != nil {
			_ = os.RemoveAll(dir)
			return fmt.Errorf("insert post images: %w", err)
		}
		// Chrome may read these files asynchronously while uploading. Keep them
		// until another draft replaces them or the browser is closed.
		if *retainedDir != "" {
			_ = os.RemoveAll(*retainedDir)
		}
		*retainedDir = dir
		report("Images inserted")
		report("Waiting for the image previews and composer to settle…")
		dialog, err = waitReadyPostDialog(ctx, page, true)
		if err != nil {
			return err
		}
	}
	if body != "" {
		if err := postingCheckpoint(ctx); err != nil {
			return err
		}
		report("Inserting post body…")
		// Uploading can rerender the editor; resolve it after attaching images.
		editor, err := waitPostElement(ctx, page, dialog, postBodySelector, true)
		if err != nil {
			return fmt.Errorf("find composer post body: %w", err)
		}
		if _, err := editor.Eval(`function() {
            this.focus();
            const range = document.createRange();
            range.selectNodeContents(this);
            const selection = window.getSelection();
            selection.removeAllRanges();
            selection.addRange(range);
        }`); err != nil {
			return fmt.Errorf("select composer text: %w", err)
		}
		// Rod uses CDP InsertText and input events, allowing Lexical to update
		// its own state instead of assigning innerHTML or textContent.
		if rich, ok := ctx.Value(richPostBodyKey{}).(richPostBody); ok && rich.text == body {
			accepted, err := editor.Eval(`function(text, html) {
                const clipboard = new DataTransfer();
                clipboard.setData('text/plain', text);
                clipboard.setData('text/html', html);
                const event = new ClipboardEvent('paste', { bubbles: true, cancelable: true, clipboardData: clipboard });
                this.dispatchEvent(event);
                return event.defaultPrevented;
            }`, body, rich.html)
			if err != nil {
				return fmt.Errorf("paste formatted post body: %w", err)
			}
			if !accepted.Value.Bool() {
				return errors.New("Facebook's editor did not accept the formatted post. Posting was not submitted")
			}
			inserted, err := editor.Eval(`function(text) {
                return new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(() => {
                    const normalize = value => value.replace(/\s+/g, ' ').trim();
                    resolve(normalize(this.innerText) === normalize(text));
                })));
            }`, body)
			if err != nil || !inserted.Value.Bool() {
				return errors.New("Facebook's editor did not retain the formatted post text. Posting was not submitted")
			}
		} else if err := editor.Input(body); err != nil {
			return fmt.Errorf("insert post body: %w", err)
		}
		report("Description inserted")
	}
	return nil
}

var errPostDialogReplaced = errors.New("The composer dialog was replaced while loading")

func waitReadyPostDialog(ctx context.Context, page *rod.Page, needsImages bool) (*rod.Element, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		dialog, err := waitPostElement(ctx, page, nil, postDialogSelector, true)
		if err != nil {
			return nil, fmt.Errorf("find composer dialog: %w", err)
		}
		for _, selector := range []string{postBodySelector} {
			if _, err := waitPostElement(ctx, page, dialog, selector, true); err != nil {
				return nil, err
			}
		}
		if _, err := waitPostSubmit(ctx, page, false); err != nil {
			return nil, err
		}
		if needsImages {
			if _, err := waitPostElement(ctx, page, dialog, postImagesSelector, false); err != nil {
				return nil, err
			}
		}
		err = waitPostDialogStable(dialog)
		if errors.Is(err, errPostDialogReplaced) {
			continue
		}
		if err != nil {
			return nil, err
		}
		return dialog, nil
	}
}

// Observe only the composer: unrelated feed updates must not prevent readiness.
// Required controls are resolved first; then require a quiet DOM and a settled
// dialog layout. This detects readiness, rather than sleeping after the click.
func waitPostDialogStable(dialog *rod.Element) error {
	result, err := dialog.Eval(`function() {
        const dialog = this;
        return new Promise(resolve => {
            let quiet;
            const done = stable => {
                clearTimeout(quiet); observer.disconnect();
                resolve(stable && dialog.isConnected);
            };
            const reset = () => {
                clearTimeout(quiet);
                quiet = setTimeout(() => {
                    const busy = dialog.matches('[aria-busy="true"]') ||
                        [...dialog.querySelectorAll('[aria-busy="true"], [role="progressbar"]')].some(el => el.getClientRects().length);
                    if (busy) reset(); else done(true);
                }, 600);
            };
            const observer = new MutationObserver(reset);
            observer.observe(dialog, {
                subtree:true, childList:true, characterData:true, attributes:true,
                attributeFilter:['aria-busy','hidden','contenteditable','disabled','aria-disabled']
            });
            reset();
        });
    }`)
	if err != nil {
		return fmt.Errorf("inspect composer readiness: %w", err)
	}
	if !result.Value.Bool() {
		return errPostDialogReplaced
	}
	if err := dialog.WaitStable(400 * time.Millisecond); err != nil {
		connected, inspectErr := dialog.Eval(`function() { return this.isConnected; }`)
		if inspectErr == nil && !connected.Value.Bool() {
			return errPostDialogReplaced
		}
		return fmt.Errorf("inspect composer layout: %w", err)
	}
	return nil
}

func stagePostImages(images []PostUpload) (string, []string, error) {
	dir, err := os.MkdirTemp("", "postika-post-images-*")
	if err != nil {
		return "", nil, errors.New("Could not prepare post images")
	}
	paths := make([]string, 0, len(images))
	for i, image := range images {
		data, err := base64.StdEncoding.DecodeString(image.Data)
		if err != nil {
			_ = os.RemoveAll(dir)
			return "", nil, errors.New("Could not read a selected image")
		}
		mime := http.DetectContentType(data)
		if mime != "image/jpeg" && mime != "image/png" && mime != "image/gif" && mime != "image/webp" {
			_ = os.RemoveAll(dir)
			return "", nil, errors.New("Choose JPG, PNG, WebP, or GIF images")
		}
		name := filepath.Base(image.Name)
		if name == "." || name == string(filepath.Separator) || name == "" {
			name = "image"
		}
		subdir := filepath.Join(dir, fmt.Sprint(i))
		if err := os.Mkdir(subdir, 0700); err != nil {
			_ = os.RemoveAll(dir)
			return "", nil, errors.New("Could not prepare image files")
		}
		path := filepath.Join(subdir, name)
		if err := os.WriteFile(path, data, 0600); err != nil {
			_ = os.RemoveAll(dir)
			return "", nil, errors.New("Could not prepare image files")
		}
		paths = append(paths, path)
	}
	return dir, paths, nil
}

func waitPostElement(ctx context.Context, page *rod.Page, scope *rod.Element, selector string, visible bool) (*rod.Element, error) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		var elements rod.Elements
		var err error
		if scope == nil {
			elements, err = page.Elements(selector)
		} else {
			connected, connectionErr := scope.Eval(`function() { return this.isConnected; }`)
			if connectionErr != nil || !connected.Value.Bool() {
				scope, err = waitPostElement(ctx, page, nil, postDialogSelector, true)
				if err != nil {
					return nil, err
				}
			}
			elements, err = scope.Elements(selector)
		}
		if err == nil {
			var matches []*rod.Element
			for _, element := range elements {
				if !visible {
					matches = append(matches, element)
					continue
				}
				if shown, err := element.Visible(); err == nil && shown {
					matches = append(matches, element)
				}
			}
			if len(matches) == 1 {
				return matches[0], nil
			}
			if len(matches) > 1 {
				return nil, errors.New("Multiple matching controls found in the post composer")
			}
		}
		select {
		case <-ctx.Done():
			return nil, errors.New("The expected composer control did not appear in Chrome")
		case <-ticker.C:
		}
	}
}
