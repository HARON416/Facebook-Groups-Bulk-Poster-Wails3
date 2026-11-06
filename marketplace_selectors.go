package main

// A versioned structural contract from the supplied item composer. Names passed
// to this function are internal field keys, never compared to Facebook text.
// Reject duplicate/unknown layouts rather than filling an arbitrary control.
const marketplaceStructuralControl = `(name, selector) => {
 const visible = el => el.getClientRects().length && !el.closest('[hidden],[aria-hidden="true"],header,nav,[role="banner"],[role="navigation"]') && getComputedStyle(el).visibility!=='hidden';
 const roots = [...document.querySelectorAll('main,[role="main"],[role="complementary"]')].filter(visible);
 const composers = roots.filter(root=>root.querySelector('input[type="file"][accept*="image/"]'));
 const scopes = composers.length ? composers : roots;
 if(scopes.length>1) throw new Error('Multiple Marketplace composers');
 const root=scopes[0]; if(!root)return null;
 const all = query => [...root.querySelectorAll(query)].filter(visible);
 const descriptions=all('textarea').filter(el=>!el.closest('label')?.querySelector('[role="list"]') && el.getAttribute('role')!=='combobox');
 const before=(a,b)=>Boolean(a.compareDocumentPosition(b)&Node.DOCUMENT_POSITION_FOLLOWING);
 let matches=[];
 switch(name) {
  case 'Title': matches=all('input[type="text"][aria-required="true"]:not([autocomplete="off"])'); break;
  case 'Price': matches=all('input[type="text"][aria-required="true"][autocomplete="off"]'); break;
  case 'Category': matches=all('label[role="combobox"]:not([aria-haspopup])'); break;
  case 'Condition': {
   const controls=all('label[role="combobox"][aria-haspopup="listbox"]');
   matches=descriptions.length===1?controls.filter(el=>before(el,descriptions[0])):controls.filter(el=>el.getAttribute('aria-required')==='true');
   break;
  }
  case 'Availability': {
   const controls=all('label[role="combobox"][aria-haspopup="listbox"]');
   if(descriptions.length===1) {
    // Required-state attributes may change once Condition has a value. The
    // captured Availability section lies after Description and before tags/SKU/location.
    const description=descriptions[0];
    const boundaries=all('textarea,input[type="text"],input[role="combobox"]').filter(el=>el!==description && before(description,el));
    matches=controls.filter(el=>before(description,el) && boundaries.every(boundary=>before(el,boundary)));
   } else matches=controls.filter(el=>el.getAttribute('aria-required')!=='true');
   break;
  }
  case 'Location': matches=all('input[role="combobox"]'); break;
  case 'More details': matches=all('div[role="button"][aria-expanded]:not([aria-haspopup]),summary'); break;
  case 'Description': matches=descriptions; break;
  case 'Product tags': matches=all('textarea,input[role="combobox"]').filter(el=>el.closest('label')?.querySelector('[role="list"]')); break;
  case 'SKU': matches=all('input[type="text"][aria-describedby]:not([aria-required="true"]):not([role="combobox"])').filter(el=>{
   const ids=el.getAttribute('aria-describedby').trim().split(/\s+/);
   return ids.length>0 && ids.every(id=>{const help=document.getElementById(id);return help && root.contains(help)});
  }); break;
  case 'Promote listing after publish': case 'Hide from friends': {
   const controls=all('input[type="checkbox"][role="switch"]');
   if(controls.length!==2)throw new Error('Unsupported Marketplace switch layout');
   matches=[controls[name==='Hide from friends'?1:0]]; break;
  }
  case 'Public meetup': case 'Door pickup': case 'Door dropoff': {
   const controls=all('[role="checkbox"],input[type="checkbox"]:not([role="switch"])');
   if(controls.length!==3)throw new Error('Unsupported Marketplace meetup layout');
   matches=[controls[['Public meetup','Door pickup','Door dropoff'].indexOf(name)]]; break;
  }
 }
 matches=matches.filter(el=>el.matches(selector));
 if(matches.length>1)throw new Error('Multiple matching controls: '+name);
 return matches[0]||null;
}`

// The supplied footer contains a two-segment progress image followed by one
// action on the edit step, or Previous + Publish on the destination step. Follow
// that relationship; aria-label strings and generated IDs/classes are ignored.
const marketplaceStructuralNavigation = `() => {
 const visible=el=>el.getClientRects().length && !el.closest('[hidden],[aria-hidden="true"]') && getComputedStyle(el).visibility!=='hidden';
 const found=[];
 for(const main of [...document.querySelectorAll('main,[role="main"]')].filter(visible)) {
  for(const progress of [...main.querySelectorAll('[role="img"]')].filter(visible)) {
   if(progress.children.length!==2 || ![...progress.children].every(segment=>segment.children.length===1))continue;
   let wrapper=progress.parentElement;
   for(let depth=0;wrapper && wrapper!==main && depth<4;depth++,wrapper=wrapper.parentElement) {
    const actions=wrapper.nextElementSibling;
    if(!actions || actions.querySelector('input,textarea,[role="combobox"]'))continue;
    const buttons=[...actions.querySelectorAll('button,[role="button"]')].filter(visible);
    if(buttons.length!==1 && buttons.length!==2)continue;
    const headings=[...main.querySelectorAll('h1,[role="heading"][aria-level="1"]')].filter(visible);
    const fields=[...main.querySelectorAll('input[aria-required="true"],textarea[aria-required="true"]')].filter(visible);
    const step=buttons.length===1?1:headings.length && fields.length===0?2:0;
    if(step)found.push({main,progress,buttons,step});
    break;
   }
  }
 }
 if(found.length>1)throw new Error('Multiple Marketplace navigation regions');
 return found[0]||null;
}`
