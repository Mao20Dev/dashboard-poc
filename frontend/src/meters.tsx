import {lazy, Suspense, useState, type ReactNode} from 'react'
import {PageSkeleton} from './skeleton'
import {num} from './api'
import {Badge, Delta, ErrorBox, Icon} from './components'
import type {Finding, Meter} from './types'

const Plant=lazy(()=>import('./plant/Plant'))

const names:Record<string,string>={REAL_ANOMALY:'Consumo sin explicación',DATA_QUALITY:'Medición por revisar',EXPLAINABLE_ANOMALY:'Cambio de operación',FALSE_POSITIVE:'Parada programada'}
const open=(id:string)=>'#/meter/'+encodeURIComponent(id)
const reason=(m:Meter)=>m.anomaly_type?names[m.anomaly_type]??m.anomaly_type:m.baseline_valid?'Dentro de lo habitual':'Referencia insuficiente'
const states=[['all','Todos'],['CRITICAL','Críticos'],['ALERT','Alertas'],['NORMAL','Normales']]

export function Meters({items,findings,controls,analysisStatus,pending,error}:{items:Meter[];findings:Finding[];controls:ReactNode;analysisStatus:ReactNode;pending:boolean;error:unknown}){
 const [sort,setSort]=useState<{key:'meter_id'|'severity'|'consumption'|'baseline'|'variation'|'share';direction:'asc'|'desc'}>({key:'severity',direction:'desc'})
 const [search,setSearch]=useState('')
 const [filter,setFilter]=useState('all')
 const [view,setView]=useState<'table'|'spatial'>('table')
 const filtered=items.filter(m=>m.meter_id.toLowerCase().includes(search.trim().toLowerCase())&&(filter==='all'||m.status===filter))
 const value=(m:Meter)=>sort.key==='meter_id'?m.meter_id:sort.key==='severity'?m.priority:sort.key==='baseline'?(m.baseline_valid?m.baseline_kwh:null):sort.key==='variation'?(m.baseline_valid?m.variation_pct:null):m.consumption_kwh
 const sorted=[...filtered].sort((a,b)=>{const av=value(a),bv=value(b);if(av===null)return bv===null?a.meter_id.localeCompare(b.meter_id):1;if(bv===null)return -1;const delta=typeof av==='string'&&typeof bv==='string'?av.localeCompare(bv):Number(av)-Number(bv);return delta*(sort.direction==='asc'?1:-1)||a.meter_id.localeCompare(b.meter_id)})
 function heading(key:typeof sort.key,title:string,unit='',numeric=false){return <th className={numeric?'number':undefined} aria-sort={sort.key===key?(sort.direction==='asc'?'ascending':'descending'):'none'}><button className="sort-button" aria-label={'Ordenar por '+title.toLowerCase()} onClick={()=>setSort({key,direction:sort.key===key&&sort.direction==='desc'?'asc':sort.key===key?'desc':key==='meter_id'?'asc':'desc'})}>{title}{unit&&<small>{unit}</small>}<span aria-hidden="true">{sort.key===key?(sort.direction==='asc'?'↑':'↓'):'↕'}</span></button></th>}
 // The denominator stays the whole installation, even while searching or filtering.
 const total=items.reduce((sum,m)=>sum+m.consumption_kwh,0)
 const largest=Math.max(0,...items.map(m=>m.consumption_kwh))
 return <div className="meters-page">
  <div className="page-heading meters-heading">
   <div><span className="eyebrow">TU OPERACIÓN</span><h1>Medidores</h1><p>Tus lecturas y lo que significan, en un solo lugar.</p></div>
   <div className="meters-actions"><div className="segmented" aria-label="Vista de medidores">
    <button aria-pressed={view==='table'} className={view==='table'?'selected':''} onClick={()=>setView('table')}>Tabla</button>
    <button aria-pressed={view==='spatial'} className={view==='spatial'?'selected':''} onClick={()=>setView('spatial')}>Vista de planta</button>
   </div>{controls}</div>
  </div>
  {analysisStatus}
  <ErrorBox error={error}/>
  <section className={'panel meters-panel '+(view==='spatial'?'is-spatial':'')} aria-label="Explorar medidores">
   <div className="meters-toolbar">
    <label className="search-field"><Icon name="search" size={17}/><input aria-label="Buscar medidor" placeholder="Buscar medidor…" value={search} onChange={e=>setSearch(e.target.value)}/></label>
    <div className="meter-filters" role="group" aria-label="Filtrar medidores">{states.map(([value,title])=><button key={value} className={'meter-filter '+value.toLowerCase()+(filter===value?' selected':'')} aria-pressed={filter===value} onClick={()=>setFilter(value)}>{value!=='all'&&<i/>}{title} · {value==='all'?items.length:items.filter(m=>m.status===value).length}</button>)}</div>
    <span className="meters-period">{view==='spatial'?'Ubicación ilustrativa':'Últimas 24 h disponibles'}</span>
   </div>
   {pending?<PageSkeleton kind="table"/>:view==='spatial'?<Suspense fallback={<PageSkeleton kind="table"/>}><Plant items={filtered}/></Suspense>:<div className="table-wrap meters-table-wrap"><table className="meters-table">
    <thead><tr>{heading('meter_id','Medidor')}{heading('severity','Estado')}{heading('consumption','Consumo','kWh',true)}{heading('baseline','Referencia','kWh',true)}{heading('variation','Variación','',true)}{heading('share','Participación del consumo')}<th>Qué ocurre</th><th><span className="sr-only">Detalle</span></th></tr></thead>
    <tbody>{sorted.map(m=><tr key={m.meter_id} className={m.status.toLowerCase()} onClick={e=>{if(!(e.target as HTMLElement).closest('a'))window.location.hash=open(m.meter_id).slice(1)}}>
     <td><a className="meter-link" href={open(m.meter_id)}>{m.meter_id}</a></td><td><Badge value={m.status}/></td>
     <td className="number consumption-value">{num(m.consumption_kwh,1)}</td><td className="number muted">{m.baseline_valid?num(m.baseline_kwh,1):'—'}</td>
     <td className="number meter-variation">{m.baseline_valid?<Delta value={m.variation_pct}/>: '—'}</td>
     <td><div className="consumption-share" title="Porcentaje del consumo total. Barras comparadas con el medidor de mayor consumo."><span className="share-track" aria-hidden="true"><span style={{width:(largest?m.consumption_kwh/largest*100:0)+'%'}}/></span><span className="share-value">{num(total?m.consumption_kwh/total*100:0,1)} %</span></div></td>
     <td className="table-reason">{reason(m)}</td><td><a className="meter-open" href={open(m.meter_id)} aria-label={'Ver '+m.meter_id}><Icon name="chevron" size={17}/></a></td>
    </tr>)}</tbody>
   </table>{!filtered.length&&<Empty reset={()=>{setSearch('');setFilter('all')}}/>}</div>}
   <span className="sr-only" role="status">{filtered.length} medidores encontrados</span>
  </section>
  {!pending&&items.length>0&&<Comparisons items={items} findings={findings}/>}
 </div>
}

function Empty({reset}:{reset?:()=>void}){return <div className="meters-empty"><Icon name="search" size={24}/><h3>No encontramos medidores</h3><p>Prueba otro nombre o cambia el estado seleccionado.</p>{reset&&<button className="ghost" onClick={reset}>Limpiar filtros</button>}</div>}

function Comparisons({items,findings}:{items:Meter[];findings:Finding[]}){
 const excess=[...items].filter(m=>m.baseline_valid).sort((a,b)=>Math.abs(b.consumption_kwh-b.baseline_kwh)-Math.abs(a.consumption_kwh-a.baseline_kwh))[0]
 const quality=[...new Set(findings.filter(f=>f.type==='DATA_QUALITY').map(f=>f.meter_id))]
 return <section className="comparison-grid meters-comparisons"><h2>Qué conviene contrastar</h2><div className="comparison-insights">
  {excess?<a className="contrast-card meter-insight" href={open(excess.meter_id)}><span className="eyebrow">MAYOR DESVIACIÓN ABSOLUTA</span><h3>{excess.meter_id} · {num(Math.abs(excess.consumption_kwh-excess.baseline_kwh),1)} kWh</h3><p>{excess.consumption_kwh>=excess.baseline_kwh?'Por encima':'Por debajo'} de su referencia en las últimas 24 h. Es una señal para revisar, no una pérdida económica demostrada.</p><span className="insight-action">Revisar medidor <Icon name="arrow" size={17}/></span></a>:<article className="meter-insight"><span className="eyebrow">REFERENCIA NO DISPONIBLE</span><h3>Aún no podemos comparar</h3><p>No hay suficientes lecturas de referencia para calcular una desviación.</p></article>}
  <article className="meter-insight quality-insight"><span className="eyebrow">CALIDAD ANTES DE CONCLUIR</span><h3>{quality.join(', ')||'Sin hallazgos de calidad'}</h3><p>{quality.length?'Un consumo estable puede coexistir con saltos de corriente, voltaje o factor de potencia. Revisa el conjunto de señales.':'El análisis seleccionado no reporta hallazgos de calidad de datos.'}</p>{quality.map(id=><a className="insight-action" key={id} href={open(id)}>Revisar {id} <Icon name="arrow" size={17}/></a>)}</article>
  <article className="meter-insight"><span className="eyebrow">QUÉ REPRESENTAN ESTOS DATOS</span><h3>Consumo de una instalación productiva</h3><p>Es la hipótesis apoyada por los eventos de operación. Comparamos energía en kWh; no inferimos potencia reactiva, capacidad ni conexiones entre medidores.</p></article>
 </div></section>
}
