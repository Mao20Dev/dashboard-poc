import {choose} from './select-helper'
import {test,expect,type Page} from './fixtures'

async function enter(page:Page){await page.goto('/');await page.locator('.demo-button').click();await expect(page.locator('.home-page')).toBeVisible()}

test('table headings sort real numbers both ways and keep filters and shares',async({page})=>{
 await enter(page)
 const data=await (await page.request.get('/api/v1/meters?limit=100')).json()
 await page.getByRole('link',{name:'Medidores',exact:true}).click()
 const rows=page.locator('.meters-table tbody tr')
 const ids=()=>rows.locator('td:first-child').allTextContents()
 const share=await rows.filter({hasText:'M-109'}).locator('.share-value').innerText()
 for(const [title,key] of [['consumo','consumption_kwh'],['variación','variation_pct'],['referencia','baseline_kwh']] as const){
  await page.getByRole('button',{name:'Ordenar por '+title,exact:true}).click()
  await expect(page.locator('.meters-table th[aria-sort=descending]')).toContainText(title==='variación'?'Variación':title==='referencia'?'Referencia':'Consumo')
  const sorted=[...data.items].sort((a,b)=>b[key]-a[key]||a.meter_id.localeCompare(b.meter_id)).map(m=>m.meter_id)
  expect(await ids()).toEqual(sorted)
  await page.getByRole('button',{name:'Ordenar por '+title,exact:true}).click()
  expect(await ids()).toEqual([...data.items].sort((a,b)=>a[key]-b[key]||a.meter_id.localeCompare(b.meter_id)).map(m=>m.meter_id))
 }
 await page.getByRole('button',{name:'Ordenar por estado',exact:true}).click()
 expect((await ids())[0]).toBe('M-109')
 await page.getByRole('button',{name:'Críticos · 2',exact:true}).click()
 await expect(rows).toHaveCount(2)
 await expect(rows.filter({hasText:'M-109'}).locator('.share-value')).toHaveText(share)
 await page.getByLabel('Buscar medidor').fill('112');await expect(rows).toHaveCount(1)
 await page.getByRole('button',{name:'Vista de planta',exact:true}).click();await expect(page.locator('.eq-plant')).toHaveAttribute('data-ready','true');await expect(page.locator('.eq-plant-node')).toHaveCount(1)
 await page.getByRole('button',{name:'Tabla',exact:true}).click();await expect(rows).toHaveCount(1)
})

test('home exposes all cases, evidence and cached result; no paid regeneration',async({page})=>{
 await enter(page)
 const dashboard=await (await page.request.get('/api/v1/dashboard/summary')).json()
 const run=await (await page.request.get('/api/v1/ai/analysis/latest')).json()
 const posts:any[]=[]
 await page.route('**/api/v1/ai/analyze',async route=>{posts.push(route.request().postDataJSON());await route.fulfill({json:{...run,provider:posts.at(-1).provider}})})
 await expect(page.locator('.anomaly-table tbody tr')).toHaveCount(4)
 await expect(page.locator('.anomaly-table tbody tr').last()).toContainText('M-106')
 await expect(page.locator('.anomaly-table tbody tr').last()).toContainText('Ver por qué no escalar')
 await expect(page.locator('.analysis-metrics')).toContainText(new Intl.NumberFormat('es-CO',{maximumFractionDigits:1}).format(dashboard.evidence_score*100))
 await choose(page,'Proveedor de IA','claude')
 await page.getByRole('button',{name:'Analizar con IA',exact:true}).click()
 await expect(page.locator('.analysis-status')).toContainText('4 anomalías detectadas · 2 requieren atención prioritaria')
 expect(posts.every(p=>p.cached_only===true)).toBe(true)
 await page.locator('.analysis-overview').screenshot({path:'../analisis/analysis-overview-light.png'})
 await page.getByRole('button',{name:'Cambiar a tema oscuro'}).click()
 await page.locator('.analysis-overview').screenshot({path:'../analisis/analysis-overview-dark.png'})
 await page.setViewportSize({width:390,height:844})
 await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
 await page.locator('.anomaly-table .meter-link').first().click()
 await expect(page.locator('.meter-page h1')).toHaveText('M-109')
 await expect(page.getByRole('button',{name:'Analizar con IA',exact:true})).toBeVisible()
 await choose(page,'Proveedor de IA','openai')
 await expect.poll(()=>posts.at(-1)?.provider).toBe('openai')
 expect(posts.at(-1).cached_only).toBe(true)
})

test('new local analysis follows API stages, survives navigation and handles failures',async({page})=>{
 await enter(page)
 const original=await (await page.request.get('/api/v1/ai/analysis/latest')).json()
 let phase='PENDING',stage='QUEUED',reads=0
 const posts:any[]=[]
 const result=()=>({...original,id:'test-local-run',provider:'rules',status:phase,stage,summary:{anomalies:4,high_priority:2}})
 await page.route('**/api/v1/ai/analyze',async route=>{posts.push(route.request().postDataJSON());await route.fulfill({status:202,json:result()})})
 await page.route('**/api/v1/ai/analysis/test-local-run',async route=>{reads++;await route.fulfill({json:result()})})
 await choose(page,'Proveedor de IA','rules')
 await expect(page.locator('.analysis-status')).toContainText('Análisis en cola')
 expect(posts[0]).toEqual({provider:'rules',cached_only:false})
 await expect(page.getByRole('button',{name:'Analizar con IA',exact:true})).toBeDisabled()
 phase='RUNNING';stage='DETECTION'
 await expect(page.locator('.analysis-stages [aria-current=step]')).toHaveText('Detectando señales')
 await page.getByRole('link',{name:'Medidores',exact:true}).click()
 stage='EXPLANATION'
 await expect(page.locator('.analysis-stages [aria-current=step]')).toHaveText('Preparando investigación')
 phase='COMPLETED';stage='COMPLETED'
 await expect(page.locator('.analysis-status')).toContainText('4 anomalías detectadas')
 await expect(page.getByRole('button',{name:'Analizar con IA',exact:true})).toBeEnabled()
 expect(reads).toBeGreaterThan(2);expect(posts).toHaveLength(1)
 phase='FAILED'
 await page.getByRole('button',{name:'Analizar con IA',exact:true}).click()
 await expect(page.locator('.analysis-status')).toContainText('No se pudo completar el análisis')
 await expect(page.locator('.analysis-stages')).toHaveCount(0)
 await expect(page.getByRole('button',{name:'Analizar con IA',exact:true})).toBeEnabled()
})
