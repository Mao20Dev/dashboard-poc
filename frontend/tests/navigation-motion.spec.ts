import {test,expect} from '@playwright/test'

test('Sonnet follows visible entry points without requests and avoids profile controls',async({page})=>{
 let paidRequests=0
 await page.route('**/api/v1/ai/chat',async route=>{
  if(route.request().method()==='POST'){paidRequests++;await route.fulfill({status:503,json:{title:'Mock: no paid requests'}});return}
  await route.fulfill({json:{enabled:true,model:'test',daily_limit:20,used_today:0}})
 })
 await page.goto('/');await page.locator('.demo-button').click()
 await expect(page.locator('.home-question')).toBeInViewport()
 await expect(page.locator('.chat-launcher')).toHaveCount(0)
 await page.locator('.home-meters').scrollIntoViewIfNeeded()
 await expect(page.locator('.chat-launcher')).toBeInViewport()
 const y=await page.evaluate(()=>scrollY)
 await page.getByRole('button',{name:'Hablar con Sonnet'}).click()
 await expect(page.locator('.chat-panel')).toBeVisible()
 await expect(page.locator('.chat-launcher')).toHaveCount(0)
 await page.getByRole('button',{name:'Cerrar chat'}).click()
 await expect(page.getByRole('button',{name:'Hablar con Sonnet'})).toBeFocused()
 expect(await page.evaluate(()=>scrollY)).toBe(y)
 await page.evaluate(()=>scrollTo(0,0));await expect(page.locator('.chat-launcher')).toHaveCount(0)
 await page.locator('.sidebar nav a[href="#/meters"]').click()
 await expect(page.locator('.meters-table tbody tr')).toHaveCount(12)
 await expect(page.locator('.chat-launcher')).toBeVisible()
 await page.getByRole('link',{name:'Ver M-109',exact:true}).click()
 await expect(page.locator('.meter-sonnet')).toBeInViewport()
 await expect(page.locator('.chat-launcher')).toHaveCount(0)
 await page.locator('.all-signals').scrollIntoViewIfNeeded()
 await expect(page.locator('.chat-launcher')).toBeVisible()
 await page.locator('.sidebar nav a[href="#/profile"]').click()
 await page.getByLabel('Nombre visible').fill('Vista previa')
 await expect(page.locator('.profile-savebar')).toBeInViewport()
 for(const width of [1440,390]){
  await page.setViewportSize({width,height:900})
  await expect.poll(async()=>{
   const a=(await page.locator('.chat-launcher').boundingBox())!,b=(await page.locator('.profile-savebar').boundingBox())!
   return a.y+a.height<=b.y||a.x>=b.x+b.width||a.x+a.width<=b.x
  }).toBe(true)
 }
 expect(paidRequests).toBe(0)
})

test('real loading shows skeletons; navigation animates and reduced motion disables it',async({page})=>{
 let release!:()=>void
 const gate=new Promise<void>(resolve=>{release=resolve})
 await page.route('**/api/v1/meters?*',async route=>{await gate;await route.continue()})
 await page.goto('/');await page.locator('.demo-button').click()
 await expect(page.locator('.skeleton-home')).toHaveAttribute('aria-busy','true')
 await page.locator('.sidebar nav a[href="#/meters"]').click()
 await expect(page.locator('.skeleton-table')).toBeVisible()
 await expect(page.locator('.skeleton-row')).toHaveCount(12)
 release()
 await expect(page.locator('.meters-table tbody tr')).toHaveCount(12)
 await expect(page.locator('.page-skeleton')).toHaveCount(0)
 expect(await page.locator('.route-stage').evaluate(el=>getComputedStyle(el).animationName)).toBe('page-arrive')
 await page.emulateMedia({reducedMotion:'reduce'})
 await page.locator('.sidebar nav a[href="#/"]').click()
 await expect(page.locator('.home-page')).toBeVisible()
 expect(await page.locator('.route-stage').evaluate(el=>getComputedStyle(el).animationName)).toBe('none')
 expect(await page.locator('.decision-card').first().evaluate(el=>getComputedStyle(el).animationName)).toBe('none')
 await expect(page.locator('.page-skeleton')).toHaveCount(0)
})

