import type {Evidence,Finding,Meter} from './types'
import {label,num} from './api'
export const findingNames:Record<string,string>={REAL_ANOMALY:'Consumo sin explicación',DATA_QUALITY:'Medición por revisar',EXPLAINABLE_ANOMALY:'Cambio de operación',FALSE_POSITIVE:'Parada programada'}
export const findingActions:Record<string,string>={REAL_ANOMALY:'Revisar el cambio de carga',DATA_QUALITY:'Validar las lecturas',EXPLAINABLE_ANOMALY:'Contrastar con operación',FALSE_POSITIVE:'Consultar la recuperación'}
export function priorityFindings(items:Finding[]){return [...items].filter(f=>f.severity==='HIGH'&&!['RESOLVED','DISMISSED'].includes(f.status)).sort((a,b)=>b.priority-a.priority||a.meter_id.localeCompare(b.meter_id))}
export function evidenceValue(e:Evidence,value:number){return num(value,e.metric==='power_factor'?2:1)+(e.metric==='power_factor'?'':` ${e.unit||({consumption_kwh:'kWh',voltage_v:'V',current_a:'A'} as Record<string,string>)[e.metric]||''}`)}
export function keyEvidence(f:Finding,powerFactor=false){return f.evidence.find(e=>e.metric===(powerFactor||f.type==='DATA_QUALITY'?'power_factor':'consumption_kwh'))??f.evidence.find(e=>e.changed)??f.evidence[0]}
export type MeterChatCard={id:string;status:string;title:string;metric:string;value:string;reference:string;period:string;priority?:number}
export function chatCards(question:string,answer:string,sources:{meter_id?:string}[],meters:Meter[],findings:Finding[],selected:string):{cards:MeterChatCard[];priority:boolean}{
 const q=question.normalize('NFD').replace(/[\u0300-\u036f]/g,'').toLowerCase()
 const priority=/(atencion|prioridad|urgente|mas problemas|primero)/.test(q)&&!selected
 const known=new Set(meters.map(m=>m.meter_id));const cited=new Set(sources.map(s=>s.meter_id).filter(id=>id&&known.has(id)))
 // Navigation targets must exist in the API catalogue. Free model text is never a URL.
 const mentioned=[...(answer+' '+question).matchAll(/\bM-\d{3}\b/g)].map(m=>m[0]).filter(id=>cited.has(id))
 const ids=priority?priorityFindings(findings).slice(0,2).map(f=>f.meter_id):[...new Set(selected&&known.has(selected)?[selected,...mentioned]:mentioned.length?mentioned:[...cited].filter((id):id is string=>!!id))].slice(0,2)
 const cards=ids.flatMap((id,index)=>{
  const m=meters.find(m=>m.meter_id===id);if(!m)return []
  const f=findings.find(f=>f.meter_id===id);const e=f&&keyEvidence(f,/factor de potencia/.test(q))
  return [{id,status:m.status,title:f?findingNames[f.type]||label(f.type):'Lecturas del medidor',metric:e?label(e.metric):'Consumo · últimas 24 h',value:e?evidenceValue(e,e.observed):num(m.consumption_kwh,1)+' kWh',reference:e?evidenceValue(e,e.baseline):m.baseline_valid?num(m.baseline_kwh,1)+' kWh':'Sin referencia',period:e?(e.metric==='consumption_kwh'?`Suma de ${e.samples} lecturas señaladas`:`Mediana de ${e.samples} lecturas señaladas`):'Últimas 24 h disponibles',priority:priority?index+1:undefined}]
 })
 return {cards,priority}
}
export type AnswerSection={kind:'prose'|'facts'|'recommendation'|'limits';title:string;body:string}
// Only explicit headings are classified. Unrecognized prose keeps its original meaning.
export function answerSections(text:string):AnswerSection[]{
 const sections:AnswerSection[]=[];let current:AnswerSection={kind:'prose',title:'',body:''}
 const flush=()=>{if(current.body.trim()||current.title)sections.push({...current,body:current.body.trim()})}
 for(const line of text.split('\n')){
  const cleaned=line.replace(/^\s*(?:#{1,3}\s*|\*\*)/,'').replace(/\*\*/g,'').trim()
  const match=cleaned.match(/^(Hechos(?: del medidor seleccionado| del medidor| de [A-Z0-9-]+| \([^\n)]{1,80}\))?|Datos clave|Lo que muestran los datos|Recomendaci[oó]n(?:es)?|Qu[eé] revisar(?: primero)?|Siguiente paso|L[ií]mites(?: del an[aá]lisis| e hip[oó]tesis)?|Lo que no sabemos|Hip[oó]tesis|Interpretaci[oó]n)\s*(?::\s*(.*)|\s*)$/i)
  if(match){flush();const title=match[1];current={kind:/^(hechos|datos|lo que muestran)/i.test(title)?'facts':/^(recomend|qu[eé] revisar|siguiente)/i.test(title)?'recommendation':'limits',title,body:match[2]||''}}
  else current.body+=(current.body?'\n':'')+line
 }
 flush();return sections
}
