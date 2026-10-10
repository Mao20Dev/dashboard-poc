import {useEffect,useRef,useState,type CSSProperties} from 'react'
import {createPortal} from 'react-dom'
import {Badge,Delta} from '../components'
import {num} from '../api'
import type {Meter} from '../types'
import {createPlantViewer} from './viewer'
import anchors from './anchors.json'
import './plant.css'

const pairs=[['M-101','M-102'],['M-102','M-103'],['M-103','M-105'],['M-105','M-104'],['M-104','M-106'],['M-106','M-112'],['M-112','M-107'],['M-107','M-110'],['M-107','M-109'],['M-109','M-108'],['M-109','M-111'],['M-111','M-101']]
const reasons:Record<string,string>={REAL_ANOMALY:'Consumo sin explicación',DATA_QUALITY:'Medición por revisar',EXPLAINABLE_ANOMALY:'Cambio de operación',FALSE_POSITIVE:'Parada programada'}
const href=(id:string)=>'#/meter/'+encodeURIComponent(id)
const color=(m:Meter)=>m.status==='CRITICAL'?'#ff746a':m.status==='ALERT'?'#efbc63':m.status==='NORMAL'?'#a4eaa1':'#b6c1c3'
export default function Plant({items}:{items:Meter[]}){
 const root=useRef<HTMLDivElement>(null),host=useRef<HTMLDivElement>(null),card=useRef<HTMLDivElement>(null),viewer=useRef<ReturnType<typeof createPlantViewer>|null>(null)
 const timer=useRef<ReturnType<typeof setTimeout>|undefined>(undefined),touch=useRef(false),pinned=useRef(false),point=useRef({x:0,y:0})
 const [active,setActive]=useState<string|null>(null),[status,setStatus]=useState('loading'),[connections,setConnections]=useState(true)
 const selected=items.find(m=>m.meter_id===active)
 function cancel(){clearTimeout(timer.current)}
 function close(){cancel();pinned.current=false;setActive(null)}
 function delayClose(){if(pinned.current)return;cancel();timer.current=setTimeout(close,220)}
 function position(x:number,y:number){point.current={x,y};const el=card.current;if(!el)return;const {width,height}=el.getBoundingClientRect();const left=x+22+width>innerWidth-12?x-width-22:x+22;el.style.left=Math.max(12,Math.min(innerWidth-width-12,left))+'px';el.style.top=Math.max(12,Math.min(innerHeight-height-12,y-35))+'px'}
 function show(id:string,x:number,y:number){cancel();point.current={x,y};setActive(id);position(x,y)}
 useEffect(()=>{try{viewer.current=createPlantViewer(root.current!,host.current!,()=>setStatus('ready'),()=>setStatus('error'),close)}catch{setStatus('error')}return()=>{cancel();viewer.current?.dispose();viewer.current=null}},[])
 useEffect(()=>{close();viewer.current?.refresh()},[items.map(m=>m.meter_id).join(',')])
 useEffect(()=>{position(point.current.x,point.current.y)},[active])
 useEffect(()=>{
  const escape=(e:KeyboardEvent)=>{if(e.key==='Escape')close()}
  const scroll=()=>{const focused=document.activeElement;if(focused instanceof HTMLElement&&focused.matches('[data-plant-id]')){const r=focused.getBoundingClientRect();position(r.right,r.top);return}if(card.current?.contains(focused))return;close()}
  window.addEventListener('keydown',escape);window.addEventListener('scroll',scroll,true)
  return()=>{window.removeEventListener('keydown',escape);window.removeEventListener('scroll',scroll,true)}
 },[])
 return <>
  <div ref={root} className={'eq-plant '+(status==='ready'?'is-ready':'')} data-scene="plant" data-render-mode="baked-3d" data-ready={status==='ready'}>
   <div ref={host} className="eq-plant-model"/>
   {status!=='ready'&&<div className="eq-plant-loading" role="status">{status==='error'?<><strong>No pudimos abrir la planta 3D.</strong><span>Puedes consultar todos los medidores en la vista Tabla.</span></>:<><span className="eq-plant-loader"/><strong>Abriendo la planta…</strong><span>Preparando tu vista de los medidores.</span></>}</div>}
   <svg className="eq-plant-links" viewBox="0 0 1800 1160" preserveAspectRatio="none" aria-hidden="true" style={{opacity:connections?1:0,'--signal':selected?color(selected):'#a4eaa1'} as CSSProperties}>{pairs.filter(([a,b])=>items.some(m=>m.meter_id===a)&&items.some(m=>m.meter_id===b)).map(([a,b])=><path key={a+b} data-from={a} data-to={b} className={active===a||active===b?'is-active':''}/>)}</svg>
   {items.map(m=>{const a=anchors.find(a=>a.id===m.meter_id);if(!a)return null;return <a key={m.meter_id} href={href(m.meter_id)} data-plant-id={m.meter_id} className={'eq-plant-node '+m.status+(active===m.meter_id?' active':'')} style={{'--signal':color(m),'--pulse':m.status==='CRITICAL'?'3s':m.status==='ALERT'?'3.8s':'4.8s'} as CSSProperties} aria-label={'Ver medidor '+m.meter_id} aria-expanded={active===m.meter_id}
    onPointerEnter={e=>{if(e.pointerType!=='touch'){pinned.current=false;show(m.meter_id,e.clientX,e.clientY)}}} onPointerMove={e=>{if(active===m.meter_id&&e.pointerType!=='touch')position(e.clientX,e.clientY)}} onPointerLeave={delayClose}
    onFocus={e=>{const r=e.currentTarget.getBoundingClientRect();show(m.meter_id,r.right,r.top)}} onBlur={delayClose} onPointerDown={e=>{touch.current=e.pointerType==='touch';pinned.current=touch.current}} onClick={e=>{if(touch.current){e.preventDefault();const r=e.currentTarget.getBoundingClientRect();show(m.meter_id,r.right,r.top)}}}>
    <span className="eq-plant-wave" aria-hidden="true"/><span className="eq-plant-face"><svg viewBox="0 0 24 24" aria-hidden="true"><rect x="5" y="3" width="14" height="18" rx="2"/><path d="M8 7h8v6H8zM8 17h2m4 0h2M10 10h4"/></svg></span><span className="eq-plant-label">{m.meter_id}</span>
   </a>})}
   {status==='ready'&&!items.length&&<div className="eq-plant-empty">No hay medidores con estos filtros.</div>}
   <div className="eq-plant-tools"><button aria-label="Mostrar conexiones ilustrativas" aria-pressed={connections} onClick={()=>setConnections(!connections)}>Conexiones</button><span/><button aria-label="Alejar planta" onClick={()=>viewer.current?.zoom(-.12)}>−</button><button aria-label="Restablecer vista de planta" onClick={()=>viewer.current?.reset()}>Centrar</button><button aria-label="Acercar planta" onClick={()=>viewer.current?.zoom(.12)}>+</button></div>
   <p className="eq-plant-hint">Arrastra para explorar · Selecciona un medidor</p>
  </div>
  <p className="eq-plant-disclaimer">Estados y lecturas del análisis seleccionado. Distribución y conexiones ilustrativas; no representan el circuito eléctrico real.</p>
  {selected&&status==='ready'&&createPortal(<div ref={card} className="eq-plant-card" role="region" aria-label={'Resumen de '+selected.meter_id} onPointerEnter={cancel} onPointerLeave={delayClose} onFocus={cancel} onBlur={e=>{if(!e.currentTarget.contains(e.relatedTarget))delayClose()}}>
   <div className="eq-plant-card-title"><strong>{selected.meter_id}</strong><Badge value={selected.status}/><button aria-label="Cerrar resumen del medidor" onClick={close}>×</button></div>
   <small>{anchors.find(a=>a.id===selected.meter_id)?.zone} · ubicación ilustrativa</small>
   <p>{selected.anomaly_type?reasons[selected.anomaly_type]??'Revisar hallazgo':selected.baseline_valid?'Dentro de lo habitual':'Referencia insuficiente'}</p>
   <dl><div><dt>Últimas 24 h</dt><dd>{num(selected.consumption_kwh,1)} <small>kWh</small></dd></div><div><dt>Referencia</dt><dd>{selected.baseline_valid?num(selected.baseline_kwh,1):'—'} <small>kWh</small></dd></div><div><dt>Variación</dt><dd>{selected.baseline_valid?<Delta value={selected.variation_pct}/>: 'Sin referencia'}</dd></div></dl>
   <a href={href(selected.meter_id)}>Abrir medidor <span aria-hidden="true">→</span></a>
  </div>,document.body)}
 </>
}
