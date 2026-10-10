import {Badge,Icon} from './components'
import {num} from './api'
import {askSonnet} from './meter-page'
import {evidenceValue,findingActions,findingNames,keyEvidence,priorityFindings} from './chat-presentation'
import type {Finding} from './types'
export function DecisionCards({findings}:{findings:Finding[]}){
 const priorities=priorityFindings(findings).slice(0,2)
 return <section className="decision-grid" aria-label="Prioridades de la operación">
  {priorities.map((f,i)=>{const e=keyEvidence(f);return <article className={'decision-card '+(i===0?'contrast-card':'')} key={f.id}>
   <div className="decision-top"><span><i className="priority-dot"/>PRIORIDAD {i+1}</span><Badge value={f.severity}/></div>
   <a className="decision-title" href={'#/meter/'+f.meter_id}><h2>{f.meter_id}</h2><Icon name="arrow" size={22}/></a><p>{findingNames[f.type]}</p>
   {e&&<div className="decision-measure"><div><span>{e.metric==='power_factor'?'Factor de potencia · mediana':'Energía en lecturas señaladas'}</span><strong>{evidenceValue(e,e.observed)}</strong></div><div><span>Referencia</span><strong>{evidenceValue(e,e.baseline)}</strong></div></div>}
   <div className="decision-evidence"><span>Índice de evidencia <b>{num(f.confidence*100,0)}/100</b></span><span className="evidence-meter" aria-hidden="true"><i style={{width:f.confidence*100+'%'}}/></span><small>Apoyo de las señales; no certeza de avería.</small></div>
   <a className="decision-action" href={'#/meter/'+f.meter_id}>{findingActions[f.type]||'Revisar hallazgo'}<Icon name="arrow" size={16}/></a>
  </article>})}
  {!priorities.length&&<article className="decision-card"><Icon name="check"/><h2>Sin prioridades altas abiertas</h2><p>Consulta los medidores para revisar sus lecturas.</p><a className="inline-link" href="#/meters">Ver medidores →</a></article>}
  <article className="assistant-home"><div className="assistant-home-top"><span><Icon name="spark" size={16}/> Asistente de energía</span><small>Sonnet</small></div><span className="assistant-orb"><Icon name="spark" size={28}/></span><h2>¿Por dónde empiezo?</h2><p>Pregunta sobre tus datos.<br/>Llega directo al medidor.</p><button onClick={()=>askSonnet('¿Qué medidor merece atención primero?')}>Muéstrame las prioridades <Icon name="arrow" size={16}/></button><button className="text-button" onClick={()=>askSonnet('¿Qué cambió en el consumo?')}>Entender el consumo ↗</button></article>
 </section>
}
export function FindingClasses({findings}:{findings:Finding[]}){
 const types=[['REAL_ANOMALY','Sin explicación','Cambio que los eventos no explican.'],['DATA_QUALITY','Revisar medición','Lecturas que necesitan validación.'],['EXPLAINABLE_ANOMALY','Con contexto','Cambio coincidente con la operación.'],['FALSE_POSITIVE','Recuperado','Reducción explicada por una parada.']]
 return <div className="finding-classes" aria-label="Tipos de hallazgo">{types.map(([type,title,description])=>{const matches=findings.filter(f=>f.type===type);return <details key={type}><summary><span className={'class-dot '+type.toLowerCase()}/><span>{title}</span><b>{matches.length}</b></summary><p>{description}</p>{matches.map(f=><a href={'#/meter/'+f.meter_id} key={f.id}>{f.meter_id} · Ver evidencia ↗</a>)}</details>})}</div>
}
