import {useEffect,useId,useRef,useState} from 'react'
import {Select} from './select'
import {num} from './api'
import {shortDate} from './components'
import type {Finding,Reading} from './types'

export const signalMetrics=[
 {key:'current_a',name:'Corriente',unit:'A',baseline:'baseline_current_a'},
 {key:'voltage_v',name:'Voltaje',unit:'V',baseline:'baseline_voltage_v'},
 {key:'active_power',name:'Potencia activa',unit:'kW',baseline:'baseline_kwh'},
 {key:'power_factor',name:'Factor de potencia',unit:'sin unidad',baseline:'baseline_power_factor'},
] as const
export const energyMetric={key:'consumption_kwh',name:'Consumo',unit:'kWh',baseline:'baseline_kwh'} as const
export type Metric=typeof signalMetrics[number]|typeof energyMetric
export const measured=(r:Reading,m:Metric)=>m.key==='active_power'?r.consumption_kwh:r[m.key]
export const expected=(r:Reading,m:Metric)=>r[m.baseline]
export const localDay=(s:string)=>new Intl.DateTimeFormat('en-CA',{timeZone:'America/Bogota',year:'numeric',month:'2-digit',day:'2-digit'}).format(new Date(s))
const dayLabel=(s:string)=>new Date(s).toLocaleDateString('es-CO',{timeZone:'America/Bogota',day:'numeric',month:'short'})
export function SignalLegend(){return <div className="signal-legend"><span><i/>Medido</span><span><i className="reference"/>Lo habitual</span><span><i className="window"/>Periodo con señal</span></div>}

function nearest(points:Reading[],time:number){let lo=0,hi=points.length-1;while(lo<hi){const mid=Math.floor((lo+hi)/2);if(Date.parse(points[mid].timestamp)<time)lo=mid+1;else hi=mid}return lo>0&&time-Date.parse(points[lo-1].timestamp)<Date.parse(points[lo].timestamp)-time?lo-1:lo}

export function SignalPlot({points,metric,finding,active,onSelect,tall=false,reference=true}:{points:Reading[];metric:Metric;finding?:Finding;active:number|null;onSelect:(i:number|null)=>void;tall?:boolean;reference?:boolean}){
 const host=useRef<HTMLDivElement>(null);const [width,setWidth]=useState(700);const uid=useId().replace(/:/g,'')
 useEffect(()=>{const el=host.current;if(!el)return;const observer=new ResizeObserver(([entry])=>setWidth(Math.max(150,entry.contentRect.width)));observer.observe(el);return()=>observer.disconnect()},[])
 if(!points.length)return <p className="muted">No hay lecturas disponibles.</p>
 const height=tall?290:126,left=44,right=12,top=15,bottom=28,plotW=width-left-right,plotH=height-top-bottom
 const start=Date.parse(points[0].timestamp),end=Date.parse(points.at(-1)!.timestamp)
 const values=points.flatMap(r=>[measured(r,metric),...(reference&&expected(r,metric)!==undefined?[expected(r,metric)!]:[])]).filter(Number.isFinite)
 const lowest=Math.min(...values),highest=Math.max(...values),padding=Math.max((highest-lowest)*.12,highest*.015,.01)
 const low=Math.max(0,lowest-padding),high=highest+padding
 const x=(time:number)=>left+(time-start)/(end-start||1)*plotW
 const y=(value:number)=>top+(high-value)/(high-low||1)*plotH
 const path=(baseline:boolean)=>{
  let d='',previous:number|undefined
  points.forEach(r=>{const value=baseline?expected(r,metric):measured(r,metric),time=Date.parse(r.timestamp);if(value===undefined||!Number.isFinite(value)){previous=undefined;return}d+=(previous===undefined||time-previous>3600000?'M':'L')+x(time).toFixed(2)+','+y(value).toFixed(2)+' ';previous=time})
  return d
 }
 const from=finding?Math.max(start,Date.parse(finding.start)):0,to=finding?Math.min(end,Date.parse(finding.end)):0
 const windowVisible=!!finding&&to>=from
 const changed=finding?.evidence.find(e=>e.metric===(metric.key==='active_power'?'consumption_kwh':metric.key))?.changed
 const selected=active!==null?points[active]:undefined
 function pick(clientX:number){if(!host.current)return;const rect=host.current.getBoundingClientRect();const ratio=Math.max(0,Math.min(1,(clientX-rect.left-left)/plotW));onSelect(nearest(points,start+ratio*(end-start)))}
 const ticks=width<440?[0,1]:[0,.25,.5,.75,1]
 return <div ref={host} className={'signal-plot '+(tall?'tall':'')} tabIndex={0} role="slider" aria-label={metric.name+': explorar lecturas por hora'} aria-valuemin={0} aria-valuemax={points.length-1} aria-valuenow={active??points.length-1} aria-valuetext={shortDate((selected??points.at(-1)!).timestamp)+' · '+num(measured(selected??points.at(-1)!,metric),2)+' '+metric.unit} onPointerMove={e=>pick(e.clientX)} onPointerDown={e=>pick(e.clientX)} onKeyDown={e=>{if(['ArrowLeft','ArrowRight','Home','End','Escape'].includes(e.key)){e.preventDefault();const current=active??points.length-1;onSelect(e.key==='Escape'?null:e.key==='Home'?0:e.key==='End'?points.length-1:Math.max(0,Math.min(points.length-1,current+(e.key==='ArrowRight'?1:-1))))}}}>
  <svg width="100%" height={height} viewBox={`0 0 ${width} ${height}`} aria-hidden="true">
   <defs><clipPath id={uid}><rect x={windowVisible?x(from):0} y={top} width={windowVisible?Math.max(1,x(to)-x(from)):0} height={plotH}/></clipPath></defs>
   {[low,...(tall?[(low+high)/2]:[]),high].map((value,i)=><g key={i}><line x1={left} x2={width-right} y1={y(value)} y2={y(value)} stroke="var(--line)"/><text x={left-7} y={y(value)+4} textAnchor="end" fill="var(--muted)" fontSize="13">{num(value,metric.key==='power_factor'?2:0)}</text></g>)}
   {windowVisible&&<rect className="signal-window" x={x(from)} y={top} width={Math.max(1,x(to)-x(from))} height={plotH} fill="var(--signal-color)" opacity=".10"/>}
   {reference&&<path d={path(true)} fill="none" stroke="var(--muted)" strokeDasharray="4 4" strokeWidth="1.2"/>}
   <path d={path(false)} fill="none" stroke="var(--meter-good)" strokeWidth="1.6"/>
   {windowVisible&&changed&&<path d={path(false)} fill="none" stroke="var(--signal-color)" strokeWidth="1.7" clipPath={`url(#${uid})`}/>}
   {tall&&windowVisible&&<line x1={x(from)} x2={x(from)} y1={top} y2={top+plotH} stroke="var(--signal-color)" strokeDasharray="3 3"/>}
   {ticks.map(t=><text key={t} x={left+t*plotW} y={height-5} textAnchor={t===0?'start':t===1?'end':'middle'} fontSize="13" fill="var(--muted)">{dayLabel(new Date(start+t*(end-start)).toISOString())}</text>)}
   {selected&&<g className="shared-crosshair" data-timestamp={selected.timestamp}><line x1={x(Date.parse(selected.timestamp))} x2={x(Date.parse(selected.timestamp))} y1={top} y2={top+plotH} stroke="var(--text)" strokeWidth="1" strokeDasharray="3 3"/><circle cx={x(Date.parse(selected.timestamp))} cy={y(measured(selected,metric))} r="4" fill="var(--panel)" stroke="var(--text)" strokeWidth="2"/></g>}
  </svg>
 </div>
}

export function AllSignals({points,finding,reference}:{points:Reading[];finding?:Finding;reference:boolean}){
 const [active,setActive]=useState<number|null>(null)
 const selected=points[active??points.length-1]
 return <section className="detail-panel all-signals" onMouseLeave={()=>setActive(null)}>
  <div className="detail-panel-heading"><div><h2>Todas las señales</h2><p>La misma hora en las cuatro gráficas. Pasa el cursor, toca o usa ← →.</p></div><SignalLegend/></div>
  <div className="signal-time"><span>{selected?shortDate(selected.timestamp):'Sin lecturas'} · {active===null?'última lectura':'hora seleccionada'}</span>{active!==null&&<button className="text-button" onClick={()=>setActive(null)}>Volver a última lectura</button>}</div>
  {signalMetrics.map(metric=>{
   const value=selected?measured(selected,metric):undefined,baseline=selected&&reference?expected(selected,metric):undefined
   const inWindow=selected&&finding&&Date.parse(selected.timestamp)>=Date.parse(finding.start)&&Date.parse(selected.timestamp)<Date.parse(finding.end)
   const changed=inWindow&&finding.evidence.find(e=>e.metric===(metric.key==='active_power'?'consumption_kwh':metric.key))?.changed
   return <div className={'signal-row '+(changed?'signal-changed':'')} key={metric.key} data-metric={metric.key}>
    <div className="signal-value"><span>{metric.name} <small>{metric.unit}</small></span>{metric.key==='active_power'&&<small>Media horaria</small>}<strong data-testid={'signal-value-'+metric.key}>{value===undefined?'—':num(value,metric.key==='power_factor'?2:1)} <small>{metric.unit==='sin unidad'?'':metric.unit}</small></strong><p>Habitual <b>{baseline===undefined?'—':num(baseline,metric.key==='power_factor'?2:1)}</b> {metric.unit==='sin unidad'?'':metric.unit}</p></div>
    <SignalPlot points={points} metric={metric} finding={finding} active={active} onSelect={setActive} reference={reference}/>
   </div>
  })}
  <p className="detail-caption">Potencia activa media horaria = energía del intervalo (kWh) / 1 h. No es potencia instantánea.</p>
 </section>
}

export function Timeline({points,finding,reference}:{points:Reading[];finding?:Finding;reference:boolean}){
 const [variable,setVariable]=useState('consumption_kwh');const [active,setActive]=useState<number|null>(null)
 const metric=[energyMetric,...signalMetrics].find(m=>m.key===variable)??energyMetric
 const selected=active===null?undefined:points[active]
 return <section className="detail-panel meter-timeline" onMouseLeave={()=>setActive(null)}><div className="detail-panel-heading"><div><h2>{finding?'Cuándo empezó':'Histórico de lecturas'}</h2><p>{metric.name} por hora · {metric.unit} · hora de Colombia</p></div><div className="timeline-controls"><SignalLegend/><Select label="Variable del gráfico" value={variable} onChange={setVariable} options={[energyMetric,...signalMetrics].map(m=>({value:m.key,label:m.name,description:m.unit==='sin unidad'?'Sin unidad':m.unit}))}/></div></div>
  <div className="timeline-note">{selected?<><strong>{shortDate(selected.timestamp)}</strong><span>Medido: {num(measured(selected,metric),2)} {metric.unit}</span><span>Habitual: {reference&&expected(selected,metric)!==undefined?num(expected(selected,metric)!,2):'—'} {metric.unit}</span></>:finding?<><strong>Señal desde {shortDate(finding.start)}</strong><span>{finding.type==='DATA_QUALITY'?'Lecturas eléctricas que requieren validación.':finding.type==='FALSE_POSITIVE'?'Cambio temporal con contexto operativo.':'El consumo se aparta de su referencia.'}</span></>:<span>Compara cada lectura con lo habitual a esa misma hora.</span>}</div>
  <SignalPlot points={points} metric={metric} finding={finding} active={active} onSelect={setActive} tall reference={reference}/>
 </section>
}
