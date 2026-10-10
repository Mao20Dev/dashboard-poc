import {test,expect,type Page} from './fixtures'

async function enter(page:Page){
 await page.goto('/')
 await page.getByRole('button',{name:'Entrar a la demo',exact:true}).click()
 await expect(page.locator('.home-page')).toBeVisible()
}

test('meter filters intersect search and keep the installation share denominator',async({page})=>{
 await enter(page)
 await page.getByRole('link',{name:'Medidores',exact:true}).click()
 const rows=page.locator('.meters-table tbody tr')
 await expect(rows).toHaveCount(12)
 const firstShare=await rows.filter({hasText:'M-109'}).locator('.share-value').innerText()
 await page.getByRole('button',{name:'Críticos · 2',exact:true}).click()
 await expect(rows).toHaveCount(2)
 await expect(rows.filter({hasText:'M-109'}).locator('.share-value')).toHaveText(firstShare)
 await page.getByLabel('Buscar medidor').fill('M-101')
 await expect(rows).toHaveCount(0)
 await expect(page.getByText('No encontramos medidores')).toBeVisible()
 await page.getByRole('button',{name:'Limpiar filtros'}).click()
 await expect(rows).toHaveCount(12)
 await page.getByRole('button',{name:'Normales · 9'}).click()
 await expect(rows).toHaveCount(9)
 await page.getByRole('button',{name:'Alertas · 1'}).click()
 await expect(rows).toHaveCount(1)
 await expect(rows).toContainText('M-104')
 await page.getByRole('button',{name:'Vista de planta',exact:true}).click();await expect(page.locator('.eq-plant')).toHaveAttribute('data-ready','true')
 await expect(page.locator('.eq-plant-node')).toHaveCount(1)
 await page.getByRole('link',{name:'Ver medidor M-104'}).focus()
 await expect(page.locator('.eq-plant-card')).toContainText('Cambio de operación')
 await page.keyboard.press('Escape')
 await expect(page.locator('.eq-plant-card')).toHaveCount(0)
 await page.keyboard.press('Tab')
 await page.getByRole('link',{name:'Ver medidor M-104'}).focus()
 await page.getByRole('link',{name:'Abrir medidor'}).click()
 await expect(page.locator('.meter-page h1')).toContainText('M-104')
})

test('the shared sidebar keeps its geometry on every route and when collapsed',async({page})=>{
 await page.setViewportSize({width:1600,height:1000})
 await enter(page)
 const geometry=()=>page.locator('.sidebar').evaluate(e=>{
  const nav=e.querySelector('nav')!;const s=getComputedStyle(nav.querySelector('a')!)
  return {width:e.getBoundingClientRect().width,navTop:nav.getBoundingClientRect().top,fontSize:s.fontSize,padding:s.padding}
 })
 const expanded=await geometry()
 for(const hash of ['/meters','/profile','/meter/M-109','/']){
  await page.goto('/#'+hash)
  await expect(page.locator('.workspace')).toBeVisible()
  await expect.poll(geometry).toEqual(expanded)
 }
 await page.getByRole('button',{name:'Contraer sidebar'}).click()
 await expect(page.locator('.sidebar')).toHaveCSS('width','76px')
 const collapsed=await geometry()
 expect(collapsed.width).toBeLessThan(expanded.width)
 for(const hash of ['/meters','/profile','/meter/M-112','/']){
  await page.goto('/#'+hash)
  await expect.poll(geometry).toEqual(collapsed)
 }
})

test('meter table and plant fit mobile and both themes',async({page})=>{
 await enter(page)
 await page.goto('/#/meters')
 await expect(page.locator('.meters-table tbody tr')).toHaveCount(12)
 for(const width of [1600,768,390,320]){
  await page.setViewportSize({width,height:1000})
  await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
 }
 await page.getByRole('button',{name:'Cambiar a tema oscuro'}).click()
 await expect(page.locator('html')).toHaveAttribute('data-theme','dark')
 await page.getByRole('button',{name:'Vista de planta',exact:true}).click();await expect(page.locator('.eq-plant')).toHaveAttribute('data-ready','true')
 await page.getByRole('link',{name:'Ver medidor M-109'}).focus()
 await expect(page.locator('.eq-plant-card')).toBeVisible()
 await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
 const preview=await page.locator('.eq-plant-card').boundingBox()
 expect(preview!.x).toBeGreaterThanOrEqual(0)
 expect(preview!.x+preview!.width).toBeLessThanOrEqual(320)
 await page.getByRole('link',{name:'Abrir medidor'}).click()
 await expect(page.locator('.meter-page h1')).toContainText('M-109')
})
