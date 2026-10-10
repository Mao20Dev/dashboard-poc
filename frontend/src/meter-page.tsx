import {useState,type ReactNode} from 'react'
import {useMutation,useQuery,useQueryClient} from '@tanstack/react-query'
import {api,APIError,date,label,num} from './api'
import {PageSkeleton} from './skeleton'
import {Badge,Confidence,Delta,ErrorBox,Events,Icon,Loading,shortDate} from './components'
import {AllSignals,Timeline} from './meter-charts'
import {DailyEnergy,DailySummary} from './meter-daily'
import type {Action,Event,Evidence,Finding,MeterDetailData,Page,Reading,Run} from './types'
import './meter-detail.css'

const units:Record<string,string>={consumption_kwh:'kWh',current_a:'A',voltage_v:'V',power_factor:'sin unidad'}
const source=(s:string)=>s==='claude'?'Claude Sonnet':s==='openai'?'GPT':'Motor de reglas'
export function askSonnet(message:string){window.dispatchEvent(new CustomEvent('energy-chat',{detail:{message,send:true}}))}
const closed=(f:Finding)=>['RESOLVED','DISMISSED'].includes(f.status)
function primary(findings:Finding[]){return [...findings].sort((a,b)=>Number(closed(a))-Number(closed(b))||b.priority-a.priority)[0]}
function verdict(d:MeterDetailData,f?:Finding,planned=false){
 if(!d.meter.baseline_valid)return 'Necesitamos más datos para comparar este medidor.'
 if(!f)return d.meter.status==='NOT_ANALYZED'?'Las lecturas están disponibles; falta analizarlas.':'El consumo está dentro de lo habitual.'
 if(planned)return 'El cambio fue confirmado como planificado. El caso está explicado.'
 if(closed(f))return 'El caso está cerrado. Sus lecturas y evidencia siguen disponibles.'
 if(f.type==='DATA_QUALITY')return 'Hay lecturas que no cuadran. Conviene validar la medición primero.'
 if(f.type==='FALSE_POSITIVE')return 'El consumo bajó durante una parada y después se recuperó.'
 if(f.type==='EXPLAINABLE_ANOMALY')return 'El consumo cambió y coincide con un evento de operación.'
 const energy=f.evidence.find(e=>e.metric==='consumption_kwh')
 const ratio=energy&&energy.baseline>0?energy.observed/energy.baseline:undefined
 const when=new Date(f.start).toLocaleDateString('es-CO',{timeZone:'America/Bogota',day:'numeric',month:'long'})
 if(ratio!==undefined&&ratio>=1.85&&ratio<=2.2)return `Desde el ${when}, este medidor consume cerca del doble de lo habitual.`
 return energy?.change_percent!==undefined?`El consumo ${energy.change_percent>=0?'subió':'bajó'} ${num(Math.abs(energy.change_percent),1)} % frente a lo habitual en las lecturas señaladas.`:'Hay un cambio de consumo que necesita revisión.'
}

export function MeterPage({id,technical,run,controls,analysisStatus}:{id:string;technical:boolean;run:Run|null;controls:ReactNode;analysisStatus:ReactNode}){
 const detail=useQuery({queryKey:['meter',id,run?.id],queryFn:()=>api<MeterDetailData>('/meters/'+encodeURIComponent(id))})
 const readings=useQuery({queryKey:['readings',id],queryFn:()=>api<Page<Reading>>('/meters/'+encodeURIComponent(id)+'/readings?limit=1000')})
 const events=useQuery({queryKey:['events',id],queryFn:()=>api<Page<Event>>('/meters/'+encodeURIComponent(id)+'/events')})
 const d=detail.data,points=[...(readings.data?.items??[])].sort((a,b)=>Date.parse(a.timestamp)-Date.parse(b.timestamp))
 const f=d?primary(d.anomalies):undefined
 const investigation=useQuery({queryKey:['incident',f?.id,f?.run_id],enabled:!!f,queryFn:()=>api<{finding:Finding;actions:Action[]}>('/anomalies/'+f!.id)})
 const planned=f?.status==='DISMISSED'&&!!investigation.data?.actions.some(a=>a.action==='DISMISS'&&a.note.startsWith('Planificado: '))
 const contextual=f&&(closed(f)||f.type==='FALSE_POSITIVE')
 const tone=contextual?'context':f?.severity==='HIGH'?'critical':f?.severity==='MEDIUM'?'alert':'normal'
 const noData=!readings.isPending&&!readings.error&&!points.length
 return <div className={'meter-page detail-redesign '+tone}>
  <a className="back-link" href="#/meters">← Todos los medidores</a>
  <div className="page-heading detail-heading"><div><div className="detail-title"><h1>{id}</h1>{planned?<span className="badge normal">Con contexto</span>:f?.type==='FALSE_POSITIVE'?<span className="badge normal">Recuperado</span>:d&&<Badge value={d.meter.status}/>} {f&&<Badge value={f.status}/>}</div><p>{d?`${d.meter.reading_count} lecturas · Última lectura ${shortDate(d.meter.last_reading)}`:'Cargando el medidor…'}</p></div><div className="detail-tools">{controls}<button data-sonnet-entry className="meter-sonnet" onClick={()=>askSonnet(`Explícame de forma breve qué ocurre en ${id}, qué cambió y qué no podemos concluir con estas lecturas.`)}><Icon name="spark" size={16}/>Preguntar a Sonnet sobre {id}</button></div></div>
  {analysisStatus}
  <ErrorBox error={detail.error??readings.error??events.error}/>
  {detail.isPending?<PageSkeleton kind="detail"/>:d&&<>
   {noData?<section className="detail-panel"><h2>Sin lecturas disponibles</h2><p>Verifica la comunicación del medidor. No podemos evaluar su consumo sin lecturas.</p></section>:<>
    <section className="meter-verdict"><div className="verdict-copy"><span className="eyebrow">QUÉ ESTÁ PASANDO</span><h2>{verdict(d,f,planned)}</h2><p className="ai-summary">{f?.explanation.summary.replace(/desviación sostenida de la carga/gi,'cambio de consumo que se mantiene')??(d.meter.baseline_valid?`En las últimas 24 h consumió ${num(d.meter.consumption_kwh,1)} kWh: ${num(d.meter.variation_pct,1)} % frente a su referencia. No hay hallazgos activos en el análisis seleccionado.`:'La referencia disponible no permite una comparación fiable.')}</p><div className="verdict-chips">{f?<><span>Empezó: {shortDate(f.start)}</span><span>Ventana: {num((Date.parse(f.end)-Date.parse(f.start))/3600000,1)} h</span><span>{f.related_events.some(e=>e.type!=='UNKNOWN')?'Con evento operativo registrado':'Sin evento operativo que lo explique'}</span></>:<><span>{d.meter.status==='NOT_ANALYZED'?'Análisis pendiente':d.meter.baseline_valid?'Sin señales detectadas':'Referencia insuficiente'}</span><span>{events.data?.items.length?`Último evento: ${shortDate(events.data.items.at(-1)!.timestamp)}`:'Sin eventos registrados'}</span></>}</div>{f&&<small className="narrative-source">Interpretación de {source(f.explanation.source)}{run?' · '+date(run.created_at):''}</small>}</div>
    <div className="verdict-energy"><span className="eyebrow">ÚLTIMAS 24 H</span><strong>{num(d.meter.consumption_kwh,1)} <small>kWh</small></strong><p>Lo normal a estas horas: {d.meter.baseline_valid?num(d.meter.baseline_kwh,1)+' kWh':'sin referencia suficiente'}</p>{d.meter.baseline_valid&&<div className="verdict-delta"><Delta value={d.meter.variation_pct}/></div>}
     <div className="verdict-evidence">{f?<><div><span>Índice de evidencia</span><b>{num(f.confidence*100,0)}/100</b></div><span className="verdict-track"><i style={{width:f.confidence*100+'%'}}/></span><p>Apoyo de las señales; no es certeza de avería.</p></>:<><strong>{d.meter.baseline_valid?'Sin hallazgos activos':'Pendiente de referencia'}</strong><p>Periodo de datos: {shortDate(d.meter.period_start)} — {shortDate(d.meter.last_reading)}.</p></>}</div>
    </div></section>
    {readings.isPending?<Loading/>:<Timeline points={points} finding={f} reference={d.meter.baseline_valid}/>}
    <div className="detail-comparison-grid"><Changes data={d} finding={f}/>{f?<CaseActions finding={f}/>:<section className="detail-panel calm-action"><span className="eyebrow">QUÉ HACER AHORA</span><Icon name="check" size={26}/><h2>{d.meter.baseline_valid?'Nada que hacer por ahora':'Completar la referencia'}</h2><p>{d.meter.baseline_valid?'No hay alertas que requieran intervención en el análisis seleccionado. Puedes explorar las señales y comparar sus lecturas.':'Revisa la continuidad de las lecturas antes de interpretar diferencias de consumo.'}</p></section>}</div>
    {points.length>0&&<AllSignals points={points} finding={f} reference={d.meter.baseline_valid}/>}
    <DailySummary daily={d.daily_history} points={points} finding={f} reference={d.meter.baseline_valid}/>
    {d.anomalies.length?d.anomalies.map(finding=><FindingDetails key={finding.id+finding.run_id} f={finding} technical={technical}/>):<section className="detail-panel normal-analysis"><details className="disclosure"><summary>Evidencia y límites del análisis</summary><p>No se detectaron anomalías en este medidor. No hay una explicación individual guardada de {source(run?.provider??'rules')}. La ausencia de alertas no certifica el estado físico del equipo.</p></details><details className="disclosure"><summary>Bitácora e historial</summary><p>Sin un caso de investigación asociado a este medidor.</p></details></section>}
    <div className="detail-context-grid"><DailyEnergy daily={d.daily_history} finding={f} reference={d.meter.baseline_valid}/><section className="detail-panel detail-events"><div className="detail-panel-heading"><div><h2>Eventos registrados</h2><p>Contexto operativo del medidor</p></div></div><Events items={events.data?.items??[]}/><p className="detail-caption">Registro de nuevos eventos pendiente. {f?'Puedes añadir contexto a la bitácora del caso.':''}</p></section></div>
   </>}
   <details className="disclosure readings-details"><summary>Consultar las {points.length} lecturas originales</summary><div className="table-wrap raw-readings"><table><thead><tr><th>Hora de Colombia</th><th>Energía · kWh</th><th>Corriente · A</th><th>Voltaje · V</th><th>Factor de potencia</th></tr></thead><tbody>{points.map(r=><tr key={r.timestamp}><td>{shortDate(r.timestamp)}</td><td>{num(r.consumption_kwh,2)}</td><td>{num(r.current_a,2)}</td><td>{num(r.voltage_v,2)}</td><td>{num(r.power_factor,3)}</td></tr>)}</tbody></table></div></details>
  </>}
 </div>
}

function Changes({data,finding}:{data:MeterDetailData;finding?:Finding}){
 const evidence=finding?.evidence
 const rows:Evidence[]=evidence??[{id:'recent-energy',metric:'consumption_kwh',unit:'kWh',observed:data.meter.consumption_kwh,baseline:data.meter.baseline_kwh,change_percent:data.meter.variation_pct,changed:false,delta:0,start:data.meter.period_start,end:data.meter.period_end,reference_start:'',reference_end:'',samples:24,aggregation:'sum',text:''},...data.electrical.map(e=>({...e,id:e.metric,changed:false,delta:0,start:'',end:'',reference_start:'',reference_end:'',samples:1,aggregation:'median',text:''}))]
 const ordered=['consumption_kwh','current_a','power_factor','voltage_v'].flatMap(key=>rows.filter(r=>r.metric===key))
 return <section className="detail-panel changes-panel"><div className="detail-panel-heading"><div><h2>{finding?'Qué cambió':'Cómo están sus señales'}</h2><p>{finding?'Lecturas señaladas frente a su referencia':'Últimas lecturas y referencia del medidor'}</p></div></div><div className="change-rows">{ordered.map(e=>{const max=Math.max(e.observed,e.baseline,.001);const available=data.meter.baseline_valid;const direction=!available?'Sin referencia':e.changed?(e.observed>=e.baseline?'Subió':'Bajó'):'Sin cambio';return <div key={e.id} id={finding?'evidence-'+finding.id+'-'+e.id:undefined} className={'change-row '+(e.changed?'changed':'stable')}><div><strong>{label(e.metric)}</strong><small>{units[e.metric]}{e.metric==='consumption_kwh'?' · suma':finding?' · mediana':''}</small></div><span className="direction-badge">{direction}</span><div className="comparison-pair"><div><span style={{width:e.observed/max*65+'%'}}/><b>{num(e.observed,e.metric==='power_factor'?2:1)}</b></div><div className="comparison-reference"><span style={{width:available?e.baseline/max*65+'%':'0%'}}/><small>{available?num(e.baseline,e.metric==='power_factor'?2:1):'—'}</small></div></div><div>{available&&e.change_percent!==undefined?<Delta value={e.change_percent}/>: '—'}</div></div>})}</div><p className="detail-caption">{finding?`Energía: suma de ${ordered[0]?.samples??0} lecturas señaladas. Las otras señales son medianas; la ventana puede incluir horas no señaladas.`:'La energía corresponde a las últimas 24 h; las variables eléctricas, a la última lectura disponible.'}</p></section>
}

function CaseActions({finding:f}:{finding:Finding}){
 const query=useQueryClient();const [action,setAction]=useState('');const [note,setNote]=useState('');const [saved,setSaved]=useState(false)
 const notes=useQuery({queryKey:['incident',f.id,f.run_id],queryFn:()=>api<{finding:Finding;actions:Action[]}>('/anomalies/'+f.id)})
 const current=notes.data?.finding??f
 const mutation=useMutation({mutationFn:()=>api<{finding:Finding;actions:Action[]}>('/anomalies/'+f.id+'/actions',{action,note:action==='DISMISS'?'Planificado: '+note:note,expected_version:current.version}),onError:error=>{if(error instanceof APIError&&error.status===409)void query.invalidateQueries({queryKey:['incident',f.id,f.run_id]})},onSuccess:async data=>{query.setQueryData(['incident',f.id,f.run_id],data);setAction('');setNote('');setSaved(true);await query.invalidateQueries({predicate:q=>['meter','meters-overview','findings-overview','dashboard','history'].includes(String(q.queryKey[0]))})}})
 const last=notes.data?.actions.at(-1)
 return <section className={'detail-panel case-actions '+(closed(current)||f.type==='FALSE_POSITIVE'?'calm-action':'contrast-card')}><span className="eyebrow">QUÉ HACER AHORA</span><ErrorBox error={notes.error}/>
  {closed(current)?<><h2>{current.status==='DISMISSED'?(notes.data?.actions.some(a=>a.action==='DISMISS'&&a.note.startsWith('Planificado: '))?'Cambio explicado':'Caso descartado'):'Caso resuelto'}</h2><p>{last?`${last.actor} · ${date(last.created_at)}`:'Consulta el cierre en la bitácora.'}</p><p>{last?.note}</p></>:f.type==='FALSE_POSITIVE'?<><h2>Parada con contexto operativo</h2><p>La señal histórica permanece visible para contrastar la recuperación. No se propone una intervención por este hallazgo.</p></>:<ol>{f.explanation.actions.slice(0,3).map((a,i)=><li key={a.code}><span>{i+1}</span><p>{a.text}</p></li>)}</ol>}
  {!action&&<div className="case-buttons">{closed(current)?<button onClick={()=>{setSaved(false);setAction('REOPEN')}}>Reabrir</button>:f.type!=='FALSE_POSITIVE'?<><button onClick={()=>{setSaved(false);setAction('DISMISS')}}>Fue planificado</button><button className="ghost" onClick={()=>{setSaved(false);setAction(current.status==='OPEN'?'INVESTIGATE':'NOTE')}}>{current.status==='OPEN'?'Escalar a campo':'Añadir seguimiento'}</button></>:null}<button className="text-button" onClick={()=>{setSaved(false);setAction('NOTE')}}>Añadir contexto</button></div>}
  {action&&<form className="case-action-form" onSubmit={e=>{e.preventDefault();mutation.mutate()}}><label>{action==='DISMISS'?'¿Qué operación explica el cambio?':action==='REOPEN'?'Motivo de reapertura':'Contexto y siguiente paso'}<textarea autoFocus minLength={5} maxLength={1800} required value={note} onChange={e=>setNote(e.target.value)} placeholder="Describe lo que se verificó…"/></label>{action==='DISMISS'&&<p>Se registrará como planificado y se cerrará el caso como descartado. La evidencia se conserva.</p>}{action==='INVESTIGATE'&&<p>Marca el caso en investigación y registra la nota; no envía una orden externa.</p>}<ErrorBox error={mutation.error}/><div className="case-buttons"><button disabled={mutation.isPending||note.trim().length<5}>Guardar en bitácora</button><button className="ghost" type="button" disabled={mutation.isPending} onClick={()=>setAction('')}>Cancelar</button></div></form>}
  {saved&&<p role="status">Cambio guardado en la bitácora.</p>}
 </section>
}

function FindingDetails({f,technical}:{f:Finding;technical:boolean}){
 const notes=useQuery({queryKey:['incident',f.id,f.run_id],queryFn:()=>api<{finding:Finding;actions:Action[]}>('/anomalies/'+f.id)})
 const history=useQuery({queryKey:['history',f.id,f.run_id],queryFn:()=>api<Page<Finding>>('/anomalies/'+f.id+'/history')})
 const e=f.explanation
 function refs(ids:string[]|null|undefined){return <div className="evidence-links">{(ids??[]).map(id=>{const fact=f.evidence.find(v=>v.id===id);return fact?<a key={id} href={'#evidence-'+f.id+'-'+id} onClick={event=>{event.preventDefault();document.getElementById('evidence-'+f.id+'-'+id)?.scrollIntoView({behavior:window.matchMedia('(prefers-reduced-motion: reduce)').matches?'instant':'smooth',block:'center'})}}>{label(fact.metric)} ↗</a>:null})}</div>}
 return <section className="detail-panel finding-analysis analysis-details">
  {!!e.hypotheses.length&&<details className="disclosure hypotheses" open><summary>Posibles explicaciones <span>{e.hypotheses.length} hipótesis · por verificar</span></summary><div className="hypothesis-grid">{e.hypotheses.map((g,i)=><article className="analysis-paragraph" key={g.code}><span className="eyebrow">HIPÓTESIS {i+1}</span><p>{g.text}</p>{refs(g.evidence_ids)}</article>)}</div></details>}
  {!!e.interpretation?.length&&<details className="disclosure interpretation-section"><summary>Cómo encajan las señales <span>{e.interpretation.length} observaciones</span></summary>{e.interpretation.map(g=><div className="analysis-paragraph" key={g.code}><p>{g.text}</p>{refs(g.evidence_ids)}</div>)}</details>}
  <details className="disclosure" open={technical||undefined}><summary>Evidencia, preguntas y límites del análisis</summary><div className="confidence-row"><Confidence value={f.confidence}/><small>Índice de evidencia; no es probabilidad de avería.</small></div><p>{e.uncertainty||'Las lecturas no identifican por sí solas la causa física.'}</p><ul>{f.confidence_factors.map((c,i)=><li key={i}>{c.name} · peso {num(c.weight*100,0)} % · señal {num(c.value*100,0)} %</li>)}</ul><h3>Preguntas para operaciones</h3>{e.questions.map(g=><p className="analysis-paragraph" key={g.code}>{g.text}</p>)}<ul>{e.limitations.map((l,i)=><li key={i}>{l}</li>)}</ul><p>Referencia: {shortDate(f.evidence[0]?.reference_start??f.start)} — {shortDate(f.evidence[0]?.reference_end??f.start)}.</p><p>Ventana de señal hasta {shortDate(f.end)} (fin exclusivo).</p><div className="table-wrap"><table><thead><tr><th>Señal</th><th>Medido</th><th>Referencia</th><th>Mín.–máx.</th><th>Lecturas</th></tr></thead><tbody>{f.evidence.map(v=><tr key={v.id}><td>{label(v.metric)}</td><td>{num(v.observed,2)} {v.unit}</td><td>{num(v.baseline,2)} {v.unit}</td><td>{v.minimum===undefined?'—':num(v.minimum,2)}–{v.maximum===undefined?'—':num(v.maximum,2)}</td><td>{v.samples}</td></tr>)}</tbody></table></div></details>
  <details className="disclosure"><summary>Bitácora e historial de análisis <span>{source(e.source)}</span></summary><ErrorBox error={notes.error??history.error}/>{notes.data?.actions.length?notes.data.actions.map(a=><div className="note" key={a.id}><strong>{label(a.action)} · {label(a.from_status)} → {label(a.to_status)}</strong><p>{a.note}</p><small>{a.actor} · {date(a.created_at)}</small></div>):<p>Sin notas de investigación registradas.</p>}{history.data?.items.map(h=><details key={h.run_id} className="historical-analysis"><summary>{source(h.explanation.source)} · {h.explanation.model||'Reglas'} · {h.run_id.slice(0,8)}</summary><p>{h.explanation.summary}</p></details>)}</details>
 </section>
}
