import {test,expect,type Page} from '@playwright/test'
async function enter(page:Page,id='M-109'){
 await page.goto('/');await page.locator('.demo-button').click();await expect(page.locator('.home-page')).toBeVisible()
 await page.goto('/#/meter/'+id);await expect(page.locator('.all-signals .signal-row')).toHaveCount(4)
}
test('hover synchronizes the exact timestamp and all four real values; keyboard also works',async({page})=>{
 await enter(page)
 const response=await page.request.get('/api/v1/meters/M-109/readings?limit=1000');const points=(await response.json()).items
 const plot=page.locator('.all-signals .signal-plot').first();await plot.scrollIntoViewIfNeeded();const rect=(await plot.boundingBox())!
 await page.mouse.move(rect.x+rect.width*.85,rect.y+50)
 const markers=page.locator('.all-signals .shared-crosshair')
 await expect(markers).toHaveCount(4)
 const time=await markers.first().getAttribute('data-timestamp');expect(time).toBeTruthy()
 for(const marker of await markers.all())await expect(marker).toHaveAttribute('data-timestamp',time!)
 const row=points.find((r:any)=>r.timestamp===time);expect(row).toBeTruthy()
 const fmt=(n:number,digits=1)=>new Intl.NumberFormat('es-CO',{maximumFractionDigits:digits}).format(n)
 for(const [metric,value,digits] of [['current_a',row.current_a,1],['voltage_v',row.voltage_v,1],['active_power',row.consumption_kwh,1],['power_factor',row.power_factor,2]] as const)await expect(page.getByTestId('signal-value-'+metric)).toContainText(fmt(value,digits))
 const voltPlot=page.locator('.all-signals .signal-plot').nth(1);await voltPlot.focus();await page.keyboard.press('Home')
 for(const marker of await markers.all())await expect(marker).toHaveAttribute('data-timestamp',points[0].timestamp)
 await page.keyboard.press('ArrowRight');await expect(markers.first()).toHaveAttribute('data-timestamp',points[1].timestamp)
 await page.keyboard.press('Escape');await expect(markers).toHaveCount(0)
 await expect(page.locator('.hypotheses')).toHaveAttribute('open','')
 await expect(page.locator('.daily-summary tbody tr')).toHaveCount(5)
 await page.getByRole('button',{name:'Ver los 14 días'}).click();await expect(page.locator('.daily-summary tbody tr')).toHaveCount(14)
 await expect(page.locator('.chat-launcher')).toBeVisible()
 await expect(page.getByLabel('Proveedor de IA')).toHaveCount(1)
 await expect(page.locator('.meter-sonnet')).toHaveCount(1)
})

test('normal, critical, medium, recovered and insufficient reference are truthful and responsive',async({page})=>{
 await enter(page,'M-101');await expect(page.locator('.meter-verdict')).toContainText('El consumo está dentro de lo habitual')
 await expect(page.locator('.signal-window')).toHaveCount(0);await expect(page.locator('.hypotheses')).toHaveCount(0)
 await expect(page.locator('.daily-summary tbody tr')).toHaveCount(5)
 for(const [id,tone] of [['M-109','critical'],['M-104','alert'],['M-106','context']]){
  await page.goto('/#/meter/'+id);await expect(page.locator('.detail-redesign')).toHaveClass(new RegExp(tone))
  await expect(page.locator('.all-signals .signal-row')).toHaveCount(4)
 }
 await page.goto('/#/meter/M-109')
 for(const width of [1600,768,390,320]){await page.setViewportSize({width,height:1000});await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)}
 await page.getByRole('button',{name:'Cambiar a tema oscuro'}).click();await expect(page.locator('html')).toHaveAttribute('data-theme','dark')
 const plot=page.locator('.all-signals .signal-plot').first();await plot.scrollIntoViewIfNeeded();await plot.click({position:{x:100,y:45}});await expect(page.locator('.all-signals .shared-crosshair')).toHaveCount(4)
 await page.route('**/api/v1/meters/M-102',async route=>{const result=await route.fetch();const data=await result.json();data.meter.baseline_valid=false;data.meter.status='INSUFFICIENT_DATA';await route.fulfill({json:data})})
 await page.goto('/#/meter/M-102');await expect(page.locator('.meter-verdict')).toContainText('Necesitamos más datos')
 await expect(page.locator('.verdict-energy')).toContainText('sin referencia suficiente')
})

test('case actions use versioned API and show the persisted state without changing real demo records',async({page})=>{
 let writes:any[]=[];let state:any;let actions:any[]=[]
 await page.route('**/api/v1/anomalies/*',async route=>{
  const original=await route.fetch();const data=await original.json()
  if(!data.finding)return route.fulfill({response:original})
  if(!state)state={...data.finding,status:'OPEN',version:7}
  return route.fulfill({json:{finding:state,actions}})
 })
 await page.route('**/api/v1/anomalies/*/actions',async route=>{
  const body=route.request().postDataJSON();writes.push(body)
  expect(body.expected_version).toBe(state.version)
  const from=state.status;state={...state,status:body.action==='DISMISS'?'DISMISSED':'OPEN',version:state.version+1}
  actions=[...actions,{id:String(writes.length),action:body.action,note:body.note,actor:'Prueba local',from_status:from,to_status:state.status,created_at:'2026-10-10T15:00:00Z'}]
  await route.fulfill({json:{finding:state,actions}})
 })
 await enter(page);await page.getByRole('button',{name:'Fue planificado'}).click()
 await page.getByLabel('¿Qué operación explica el cambio?').fill('Turno adicional confirmado por operaciones.')
 await page.getByRole('button',{name:'Guardar en bitácora'}).click()
 await expect(page.locator('.case-actions')).toContainText('Cambio explicado')
 expect(writes[0].action).toBe('DISMISS');expect(writes[0].note).toContain('Planificado:')
 await page.getByRole('button',{name:'Reabrir',exact:true}).click();await page.getByLabel('Motivo de reapertura').fill('Se requiere verificar el alcance del turno.')
 await page.getByRole('button',{name:'Guardar en bitácora'}).click();await expect(page.getByRole('button',{name:'Fue planificado'})).toBeVisible()
 expect(writes[1].action).toBe('REOPEN');expect(writes[1].expected_version).toBe(8)
})
