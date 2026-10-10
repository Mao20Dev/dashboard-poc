import {useState,type FormEvent,type ReactNode} from 'react'
import {Bar,BarChart,Cell,CartesianGrid,ReferenceLine,ResponsiveContainer,Tooltip,XAxis,YAxis} from 'recharts'
import {AnalysisOverview} from './analysis'
import {date,label,num} from './api'
import {Badge,Icon,Panel} from './components'
import {LoginBee} from './login-bee'
import {askSonnet} from './meter-page'
import {evidenceValue,findingActions,findingNames,keyEvidence} from './chat-presentation'
import type {Dashboard,Finding,Meter,Run} from './types'

const severity:Record<string,number>={HIGH:3,MEDIUM:2,LOW:1}
const states:Record<string,number>={CRITICAL:3,ALERT:2,NORMAL:1}
const meterURL=(id:string)=>'#/meter/'+encodeURIComponent(id)
export function PeriodRange({data}:{data?:Dashboard}){
 const formatter=new Intl.DateTimeFormat('es-CO',{timeZone:'America/Bogota',day:'numeric',month:'short',year:'numeric'})
 return <span className="period-chip" title="Periodo disponible en los datos de demostración"><Icon name="clock" size={15}/>{data?formatter.formatRange(new Date(data.period_start),new Date(new Date(data.period_end).getTime()-1)):'Cargando periodo…'}</span>
}
function priorityItems(findings:Finding[]){return findings.filter(f=>f.severity!=='LOW'&&!['RESOLVED','DISMISSED'].includes(f.status)).sort((a,b)=>(severity[b.severity]??0)-(severity[a.severity]??0)||b.confidence-a.confidence||a.meter_id.localeCompare(b.meter_id))}
function heroSummary(findings:Finding[],data:Dashboard){
 const first=priorityItems(findings)[0],e=first?.evidence.find(e=>e.metric==='consumption_kwh')
 const intro=data.high_priority?`${data.high_priority} hallazgo${data.high_priority===1?'':'s'} de prioridad alta.`:'Sin hallazgos de prioridad alta.'
 if(!first)return intro+' Consulta cómo se comportan tus medidores.'
 if(first.type==='DATA_QUALITY')return intro+` Conviene validar las lecturas de ${first.meter_id} antes de sacar conclusiones.`
 if(e?.change_percent!==undefined)return intro+` ${first.meter_id} registra ${num(Math.abs(e.change_percent),1)} % ${e.change_percent>=0?'más':'menos'} energía que su referencia en las lecturas señaladas.`
 return intro+` Revisa primero ${first.meter_id}: ${(findingNames[first.type]||'hallazgo por revisar').toLowerCase()}.`
}
function PriorityCard({finding:f,index}:{finding:Finding;index:number}){
 const e=keyEvidence(f),large=index===0
 return <a href={meterURL(f.meter_id)} aria-label={`Abrir análisis de ${f.meter_id}: ${findingNames[f.type]||label(f.type)}`} className={'decision-card '+(large?'contrast-card priority-main':'priority-compact')}>
  <div className="decision-top"><span>PRIORIDAD {index+1}</span><Badge value={f.severity}/></div>
  <div className="decision-title"><h3>{f.meter_id}</h3></div><p>{findingNames[f.type]||label(f.type)}</p>
  {large?<>{e&&<div className="decision-measure"><div><span>{e.metric==='consumption_kwh'?'Energía en lecturas señaladas':label(e.metric)+' · mediana'}</span><strong>{evidenceValue(e,e.observed)}</strong></div><div><span>Referencia</span><strong>{evidenceValue(e,e.baseline)}</strong></div></div>}<div className="decision-evidence"><span>Índice de evidencia <b>{num(f.confidence*100,0)}/100</b></span><span className="evidence-meter" aria-hidden="true"><i style={{width:Math.max(0,Math.min(100,f.confidence*100))+'%'}}/></span><small>Apoyo de las señales; no es certeza de avería.</small></div></>:e&&<p className="priority-metric">{e.metric==='power_factor'?`Factor de potencia ${num(e.observed,2)} frente a ${num(e.baseline,2)}`:e.change_percent!==undefined?`${e.change_percent>0?'+':''}${num(e.change_percent,1)} % frente a la referencia`:`${evidenceValue(e,e.observed)} frente a ${evidenceValue(e,e.baseline)}`}</p>}
  <span className="decision-action">{findingActions[f.type]||'Revisar hallazgo'}<Icon name="arrow" size={16}/></span>
 </a>
}
export function Home({name,data,meters,findings,run,chatPending,controls,analysisStatus}:{name:string;data:Dashboard;meters:Meter[];findings:Finding[];run:Run|null;chatPending:boolean;controls:ReactNode;analysisStatus:ReactNode}){
 const [question,setQuestion]=useState('')
 const priorities=priorityItems(findings),ordered=[...meters].sort((a,b)=>(states[b.status]??0)-(states[a.status]??0)||a.meter_id.localeCompare(b.meter_id))
 const daily=Object.entries(data.daily_consumption).sort(([a],[b])=>a.localeCompare(b)).map(([day,kwh])=>({day,kwh}))
 // A full 24-hour reference is comparable to a full day; never substitute a mean of observed data.
 const periodHours=(new Date(data.period_end).getTime()-new Date(data.period_start).getTime())/3600000
 const reference=periodHours>=24&&meters.length===data.meters&&meters.every(m=>m.baseline_valid&&m.reading_count===periodHours&&new Date(m.period_end).getTime()===new Date(data.period_end).getTime())?data.baseline_24h:undefined
 const provider=run?.provider==='claude'?'Claude Sonnet':run?.provider==='openai'?'GPT':'Motor de reglas'
 function submit(event:FormEvent){event.preventDefault();if(question.trim().length<2||chatPending)return;askSonnet(question.trim());setQuestion('')}
 return <div className="home-page">
  <section className="home-hero" aria-label="Resumen de la operación"><div className="home-welcome"><div className="home-intro"><span className="eyebrow">TU ENERGÍA, EN CONTEXTO</span><h1>Hola, {name.split(' ')[0]}.</h1><p>{heroSummary(findings,data)}</p><form data-sonnet-entry className="home-question" onSubmit={submit}><label className="sr-only" htmlFor="home-question">Pregúntale a Sonnet sobre tus datos</label><input id="home-question" value={question} onChange={e=>setQuestion(e.target.value)} maxLength={2000} placeholder="Pregúntale a Sonnet sobre tus datos…"/><button disabled={question.trim().length<2||chatPending}><Icon name="spark" size={15}/>{chatPending?'Consultando…':'Preguntar'}</button></form></div><LoginBee variant="home"/></div><div className="home-energy"><span className="eyebrow">ÚLTIMAS 24 H</span><strong>{num(data.consumption_24h/1000,2)} <small>MWh</small></strong><p className={data.variation_24h>10?'energy-increase':''}>{data.variation_24h>0?'+':''}{num(data.variation_24h,1)} % frente a la referencia</p></div></section>
  <section className="home-priorities" aria-label="Prioridades de la operación"><div className="home-section-heading"><h2>Prioridades</h2><span className="summary-source">{run?`Análisis de ${provider} · ${date(run.created_at)}`:'Sin análisis disponible'}</span></div><div className="home-priority-grid">{priorities.slice(0,3).map((f,index)=><PriorityCard key={f.id} finding={f} index={index}/>)}{!priorities.length&&<div className="panel home-empty"><Icon name="check"/><h3>No hay prioridades abiertas</h3><p>Consulta las lecturas y el historial de cada medidor.</p></div>}</div>{priorities.length>3&&<a className="inline-link" href="#/meters">Ver todas las prioridades →</a>}</section>
  <section className="home-indicators" aria-label="Indicadores del periodo"><article className="kpi"><span>Consumo del periodo</span><strong>{num(data.consumption_kwh/1000,1)} <small>MWh</small></strong><p>{num(data.readings,0)} lecturas</p></article><article className="kpi"><span>Requieren atención</span><strong className={data.high_priority?'has-priority':''}>{data.high_priority}</strong><p>Hallazgos de prioridad alta</p></article><article className="kpi"><span>Sin alerta</span><strong>{data.status_counts.NORMAL??0} <small>/ {data.meters}</small></strong><p>Medidores</p></article></section>
  <AnalysisOverview data={data} findings={findings} run={run} controls={controls} status={analysisStatus}/>
  <div className="overview-grid"><Panel title="Consumo energético" subtitle="Energía por día · kWh" className="energy-chart"><div className="home-chart-legend"><span><i/>Sobre la referencia</span><span><i className="reference-key"/>Referencia de 24 h</span></div><div className="chart"><ResponsiveContainer width="100%" height="100%"><BarChart data={daily} margin={{left:0,right:14,top:15,bottom:5}}><CartesianGrid stroke="var(--line)" vertical={false} strokeDasharray="3 5"/><XAxis dataKey="day" tickFormatter={s=>String(s).slice(8,10)} axisLine={false} tickLine={false} stroke="var(--muted)" fontSize={14}/><YAxis width={50} tickFormatter={v=>num(Number(v)/1000,0)+'k'} axisLine={false} tickLine={false} stroke="var(--muted)" fontSize={14}/><Tooltip cursor={{fill:'var(--hover)'}} contentStyle={{background:'var(--panel)',border:'1px solid var(--line)',borderRadius:8,color:'var(--text)'}} formatter={v=>[num(Number(v),1)+' kWh','Consumo']}/><Bar dataKey="kwh" radius={[3,3,0,0]} isAnimationActive={false}>{daily.map(d=><Cell key={d.day} fill={reference!==undefined&&d.kwh>reference?'var(--home-bar-strong)':'var(--home-bar-soft)'}/>)}</Bar>{reference!==undefined&&<ReferenceLine y={reference} stroke="var(--muted)" strokeDasharray="5 5"/>}</BarChart></ResponsiveContainer></div>{reference===undefined&&<p className="muted">Referencia diaria pendiente: faltan datos comparables.</p>}</Panel><Panel title="Lo importante" subtitle="Contexto del análisis seleccionado" className="summary-panel"><div className="summary-findings">{findings.map(f=><a key={f.id} href={meterURL(f.meter_id)} className="summary-finding"><span className={'signal '+f.severity.toLowerCase()}/><div><strong>{f.meter_id} · {findingNames[f.type]}</strong><p>{f.explanation.summary}</p></div><Icon name="chevron" size={15}/></a>)}{!findings.length&&<p>No hay hallazgos en este análisis.</p>}</div><a className="inline-link" href="#/meters">Explorar el análisis de cada medidor →</a></Panel></div>
  <section className="panel home-meters"><div className="home-section-heading"><div><h2>Estado de los medidores</h2><p>Últimas 24 h · energía y variación frente a su referencia</p></div><a className="inline-link" href="#/meters">Ver medidores <Icon name="arrow" size={15}/></a></div><div className="home-meter-grid">{ordered.map(m=><a className={'home-meter '+m.status.toLowerCase()} key={m.meter_id} href={meterURL(m.meter_id)} aria-label={`Ver ${m.meter_id} · ${label(m.status)}`}><span><b>{m.meter_id}</b><i aria-label={label(m.status)}/></span><strong>{num(m.consumption_kwh,1)} <small>kWh</small></strong><p>{m.baseline_valid?`${m.variation_pct>0?'+':''}${num(m.variation_pct,1)} %`:'Sin referencia'}</p></a>)}</div></section>
 </div>
}
