import {test,expect} from '@playwright/test'
import {answerSections,chatCards} from '../src/chat-presentation'
import type {Finding,Meter} from '../src/types'
test('answer sections keep unrecognized prose and classify explicit headings only',()=>{
 const sections=answerSections('Hechos curiosos no son una medición.\n\n**Hechos del medidor seleccionado:**\n- 220 V.\n\nRecomendación: Contrastar con operación.\n\nLímites:\nNo demuestra avería.')
 expect(sections.map(s=>s.kind)).toEqual(['prose','facts','recommendation','limits'])
 expect(sections[0].body).toContain('Hechos curiosos')
 expect(sections[1].body).toBe('- 220 V.')
 expect(sections[2].body).toBe('Contrastar con operación.')
 expect(answerSections('Hechos (M-109, consumo horario):\n- 220 V.\n\nLímites e hipótesis:\nNo demuestra avería.').map(s=>s.kind)).toEqual(['facts','limits'])
 expect(answerSections('<script>alert(1)</script>')[0].body).toBe('<script>alert(1)</script>')
})
test('priority cards follow actual rankings, reject invented targets and preserve units',()=>{
 const meters=[{meter_id:'M-101',status:'CRITICAL',consumption_kwh:400,baseline_kwh:200,baseline_valid:true},{meter_id:'M-102',status:'CRITICAL',consumption_kwh:300,baseline_kwh:250,baseline_valid:true}] as Meter[]
 const findings=[{meter_id:'M-101',severity:'HIGH',status:'OPEN',priority:70,type:'REAL_ANOMALY',evidence:[]},{meter_id:'M-102',severity:'HIGH',status:'OPEN',priority:95,type:'DATA_QUALITY',evidence:[{metric:'power_factor',observed:.74,baseline:.94,samples:16}]}] as Finding[]
 const result=chatCards('¿Qué medidor tiene más problemas?','M-999',[],meters,findings,'')
 expect(result.cards.map(c=>c.id)).toEqual(['M-102','M-101'])
 expect(result.cards[0].priority).toBe(1)
 expect(result.cards[0].value).toBe('0,74')
 expect(result.cards[0].period).toContain('16 lecturas')
 expect(result.cards[1].value).toBe('400 kWh')
 expect(chatCards('Dime algo','M-999',[{meter_id:'M-999'}],meters,findings,'').cards).toEqual([])
 findings[1].status='RESOLVED'
 expect(chatCards('¿Qué revisar primero?','',[],meters,findings,'').cards.map(c=>c.id)).toEqual(['M-101'])
})
