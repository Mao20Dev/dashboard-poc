import { label, num } from './api'
import type { Evidence, Finding, Guidance } from './types'
import { Icon, Panel } from './components'

export function EvidenceLinks({ids,evidence}:{ids:string[];evidence:Evidence[]}) {
 return <div className="evidence-links">{ids.map(id=>{
  const fact=evidence.find(e=>e.id===id)
  if(!fact)return null
  const change=fact.change_percent
  const value=change!==undefined?`${change>0?'+':''}${num(change,1)} %`:num(fact.observed)
  return <a key={id} href={'#evidence-'+id} title="Ver la medición y su referencia" onClick={event=>{event.preventDefault();document.getElementById('evidence-'+id)?.scrollIntoView({behavior:'smooth',block:'center'})}}>{label(fact.metric)} <b>{value}</b><span>↗</span></a>
 })}</div>
}
export function NarrativeSteps({items,evidence}:{items:Guidance[];evidence:Evidence[]}) {
 return <ol className="guidance">{items.map((item,i)=><li key={item.code}><span className="step-number">{i+1}</span><div>{item.text}<EvidenceLinks ids={item.evidence_ids} evidence={evidence}/></div></li>)}</ol>
}
export function AnalysisStory({finding:f}:{finding:Finding}) {
 const e=f.explanation;const generative=!!e.narrative_version
 const source=e.source==='claude'?'Claude':e.source==='openai'?'GPT':'Reglas verificables'
 return <><div className="columns story-columns"><Panel title={generative?'La lectura del caso':'Qué encontramos'} className="analyst-story" aside={<span className="source"><Icon name="spark" size={14}/>{source}</span>}>
 {e.headline&&<h2 className="analyst-headline">{e.headline}</h2>}
 <p className="finding-summary">{e.summary}</p>
 <EvidenceLinks ids={e.summary_evidence_ids??[]} evidence={f.evidence}/>
 <p className="analysis-byline">{generative?`Interpretación escrita por ${source} · ${e.model}. Las referencias enlazan a las mediciones; las causas sugeridas siguen pendientes de comprobar.`:e.source==='rules'?'Resumen generado por reglas verificables.':`Plan anterior seleccionado por ${source} y validado contra el catálogo. Ejecuta un nuevo análisis para obtener una interpretación redactada.`}</p>
 {e.fallback_reason&&<p className="notice">{e.fallback_reason==='provider_not_configured'?'No hay un proveedor generativo configurado.':'La IA no pudo entregar una interpretación válida en esta ejecución.'} Estás viendo el resumen de respaldo, no un análisis redactado por IA.</p>}
 </Panel><Panel title="Por dónde empezar" className="accent-panel"><NarrativeSteps items={e.actions} evidence={f.evidence}/></Panel></div>
 {e.interpretation?.length?<Panel title="Cómo encajan las señales" subtitle="La relación entre las lecturas y lo que podemos inferir de ellas"><div className="interpretation-list">{e.interpretation.map(item=><article key={item.code}><p>{item.text}</p><EvidenceLinks ids={item.evidence_ids} evidence={f.evidence}/></article>)}</div>{e.uncertainty&&<div className="uncertainty"><Icon name="search"/><div><strong>Lo que todavía no sabemos</strong><p>{e.uncertainty}</p></div></div>}</Panel>:null}</>
}
