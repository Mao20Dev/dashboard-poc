import type {ReactNode} from 'react'
import {date,label,num} from './api'
import {Badge,Icon} from './components'
import type {Dashboard,Finding,Run} from './types'
import './analysis.css'

export const analysisBusy=(run:Run|null|undefined)=>!!run&&['PENDING','RUNNING'].includes(run.status)
export const providerName=(run:Run|null)=>run?.provider==='claude'?'Claude Sonnet':run?.provider==='openai'?'GPT':'Motor de reglas'
export function AnalysisStatus({run,requesting=false}:{run:Run|null;requesting?:boolean}){
 const busy=analysisBusy(run)
 const stages=['READINGS','DETECTION','EXPLANATION','PERSISTENCE']
 return <div className={'analysis-status '+(run?.status==='FAILED'?'failed':'')} role="status" aria-live="polite">
  <div>{requesting||busy?<span className="spinner"/>:<Icon name={run?.status==='FAILED'?'alert':run?'check':'clock'} size={16}/>}
   <strong>{requesting?'Consultando el análisis…':!run?'Aún no hay un análisis':run.status==='COMPLETED'?'Análisis completado':run.status==='FAILED'?'No se pudo completar el análisis':run.status==='PENDING'?'Análisis en cola':label(run.stage)}</strong>
   {run&&!requesting&&<span>{providerName(run)} · {date(run.created_at)}</span>}
  </div>
  {run?.status==='COMPLETED'&&!requesting&&<p>{run.summary?.anomalies??0} anomalías detectadas · {run.summary?.high_priority??0} requieren atención prioritaria</p>}
  {busy&&run&&!requesting&&<ol className="analysis-stages" aria-label="Etapas del análisis">{stages.map((stage,i)=><li key={stage} aria-current={run.stage===stage?'step':undefined} className={run.status==='RUNNING'&&i<=stages.indexOf(run.stage)?'reached':''}>{label(stage)}</li>)}</ol>}
  {run?.status==='FAILED'&&!requesting&&<p>La ejecución no terminó. Puedes elegir otro análisis disponible o utilizar el motor de reglas.</p>}
 </div>
}

export function AnalysisOverview({data,run,controls,status,findings}:{data:Dashboard;run:Run|null;controls:ReactNode;status:ReactNode;findings:Finding[]}){
 const ordered=[...findings].sort((a,b)=>b.priority-a.priority||a.meter_id.localeCompare(b.meter_id))
 const actions:Record<string,string>={REAL_ANOMALY:'Investigar',DATA_QUALITY:'Validar medición',EXPLAINABLE_ANOMALY:'Validar operación',FALSE_POSITIVE:'Ver por qué no escalar'}
 return <section className="analysis-overview" aria-label="Análisis de la operación">
  <div className="analysis-summary panel">
   <div className="analysis-heading"><div><span className="eyebrow">DE LOS DATOS A LA ACCIÓN</span><h2>Análisis de la operación</h2></div>{controls}</div>
   <div className="analysis-metrics"><div><span>Anomalías detectadas</span><strong>{data.anomalies}</strong><small>En el resultado disponible</small></div><div><span>Confianza del análisis</span><strong>{data.anomalies?num(data.evidence_score*100,1):'—'}<small>{data.anomalies?'/100':''}</small></strong><small>Índice medio de evidencia · no es probabilidad de avería</small></div></div>
   {status}
  </div>
  <div className="panel anomaly-overview">
   <div className="analysis-heading"><div><h2>Todos los hallazgos</h2><p>Compara la evidencia y abre el caso para actuar.</p></div><span className="analysis-count">{findings.length} hallazgos</span></div>
   {analysisBusy(run)&&<p className="analysis-previous">Estos hallazgos corresponden al último resultado disponible mientras termina el análisis.</p>}
   <div className="table-wrap"><table className="anomaly-table"><thead><tr><th>Medidor</th><th>Tipo de hallazgo</th><th>Severidad</th><th>Confianza <small>índice</small></th><th>Acción recomendada</th></tr></thead><tbody>{ordered.map(f=><tr key={f.id} onClick={e=>{if(!(e.target as HTMLElement).closest('a'))location.hash='/meter/'+encodeURIComponent(f.meter_id)}}><td><a className="meter-link" href={'#/meter/'+encodeURIComponent(f.meter_id)}>{f.meter_id}</a></td><td>{label(f.type)}</td><td><Badge value={f.severity}/></td><td><span className="anomaly-confidence"><span aria-hidden="true"><i style={{width:Math.min(100,Math.max(0,f.confidence*100))+'%'}}/></span><b>{num(f.confidence*100,0)}<small>/100</small></b></span></td><td><a className="inline-link" href={'#/meter/'+encodeURIComponent(f.meter_id)}>{actions[f.type]??'Revisar'} <Icon name="arrow" size={15}/></a></td></tr>)}</tbody></table></div>
   {!findings.length&&<p className="analysis-empty">{run?.status==='COMPLETED'?'No se detectaron anomalías en este análisis.':'Los hallazgos aparecerán cuando haya un análisis completado.'}</p>}
  </div>
 </section>
}

