import type {Page} from '@playwright/test'
export async function choose(page:Page,label:string,value:string){
 const names:Record<string,string>={claude:'Claude Sonnet',openai:'GPT',rules:'Reglas verificables',current_a:'Corriente',power_factor:'Factor de potencia'}
 await page.getByRole('combobox',{name:label,exact:true}).click()
 await page.getByRole('option').filter({has:page.locator('strong',{hasText:new RegExp('^'+(names[value]??value)+'$')})}).click()
}
