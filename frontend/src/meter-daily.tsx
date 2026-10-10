import {useState} from 'react'
import {num} from './api'
import {localDay} from './meter-charts'
import type {Daily,Finding,Reading} from './types'

const mean=(values:number[])=>values.length?values.reduce((a,b)=>a+b,0)/values.length:undefined
const format=(n:number|undefined,digits=1)=>n===undefined?'—':num(n,digits)
const dayName=(day:string)=>new Date(day+'T12:00:00-05:00').toLocaleDateString('es-CO',{timeZone:'America/Bogota',day:'numeric',month:'short'})
const affected=(d:Daily,f?:Finding)=>!!f&&d.date>=localDay(f.start)&&d.date<=localDay(new Date(Date.parse(f.end)-1).toISOString())

export function DailySummary({daily,points,finding,reference}:{daily:Daily[];points:Reading[];finding?:Finding;reference:boolean}){
 const [all,setAll]=useState(false)
 const first=finding?daily.findIndex(d=>affected(d,finding)):-1
 const days=all?daily:first>=0?daily.slice(Math.max(0,first-2)).filter(d=>d.date<=localDay(new Date(Date.parse(finding!.end)-1).toISOString())):daily.slice(-5)
 const referencePoints=points.filter(r=>r.is_reference)
 const refs=referencePoints.length?referencePoints:[]
 return <section className="detail-panel daily-summary"><div className="detail-panel-heading"><div><h2>Resumen por día</h2><p>Energía acumulada y medias diarias · potencia máxima de las medias horarias</p></div><button className="text-button" onClick={()=>setAll(v=>!v)}>{all?'Ver días relevantes':`Ver los ${daily.length} días`} →</button></div>
 <div className="table-wrap"><table><thead><tr><th>Día</th><th>Energía <small>kWh</small></th><th>vs. habitual</th><th>Corriente <small>A</small></th><th>Voltaje <small>V</small></th><th>Factor de potencia</th><th>Máx. media horaria <small>kW</small></th></tr></thead><tbody>
 {days.map(d=>{const rs=points.filter(r=>localDay(r.timestamp)===d.date);const change=reference&&d.baseline_kwh>0?(d.consumption_kwh/d.baseline_kwh-1)*100:undefined;return <tr key={d.date} className={affected(d,finding)?'affected-day':''}><td>{dayName(d.date)}{!d.complete&&<small className="day-partial">{d.samples}/24 lecturas</small>}</td><td>{num(d.consumption_kwh,1)}</td><td className={affected(d,finding)?'signal-emphasis':''}>{change===undefined?'—':(change>0?'+':'')+num(change,1)+' %'}</td><td>{format(mean(rs.map(r=>r.current_a)))}</td><td>{format(mean(rs.map(r=>r.voltage_v)))}</td><td>{format(mean(rs.map(r=>r.power_factor)),2)}</td><td>{rs.length?num(Math.max(...rs.map(r=>r.consumption_kwh)),1):'—'}</td></tr>})}
 </tbody><tfoot><tr><td>Habitual*</td><td>{reference?format(mean(daily.filter(d=>d.complete).map(d=>d.baseline_kwh))):'—'}</td><td>—</td><td>{reference?format(mean(refs.flatMap(r=>r.baseline_current_a===undefined?[]:[r.baseline_current_a]))):'—'}</td><td>{reference?format(mean(refs.flatMap(r=>r.baseline_voltage_v===undefined?[]:[r.baseline_voltage_v]))):'—'}</td><td>{reference?format(mean(refs.flatMap(r=>r.baseline_power_factor===undefined?[]:[r.baseline_power_factor])),2):'—'}</td><td>{reference&&refs.length?num(Math.max(...refs.map(r=>r.baseline_kwh)),1):'—'}</td></tr></tfoot></table></div><p className="detail-caption">* Energía de un día completo; corriente, voltaje y factor de potencia medios del perfil de referencia. Los días incompletos solo suman las horas disponibles.</p></section>
}

export function DailyEnergy({daily,finding,reference}:{daily:Daily[];finding?:Finding;reference:boolean}){
 const max=Math.max(1,...daily.flatMap(d=>[d.consumption_kwh,reference?d.baseline_kwh:0]))*1.08
 return <section className="detail-panel daily-energy"><div className="detail-panel-heading"><div><h2>Consumo diario</h2><p>Energía acumulada · kWh</p></div><span className="daily-key">- - Lo habitual</span></div><div className="daily-bars" role="img" aria-label="Consumo y referencia por día, en kWh. Las cifras están en Resumen por día.">
 {daily.map(d=><div className="daily-bar-column" key={d.date} tabIndex={0} aria-label={`${dayName(d.date)}: ${num(d.consumption_kwh,1)} kWh; habitual ${reference?num(d.baseline_kwh,1)+' kWh':'no disponible'}`}><div className="daily-bar-area"><span className={'daily-bar '+(affected(d,finding)?'affected':'')} style={{height:d.consumption_kwh/max*100+'%'}}/>{reference&&<i className="daily-reference" style={{bottom:d.baseline_kwh/max*100+'%'}}/>}</div><small>{d.date.slice(-2)}</small><span className="daily-bar-tooltip">{dayName(d.date)} · {num(d.consumption_kwh,1)} kWh{reference&&<><br/>Habitual: {num(d.baseline_kwh,1)} kWh</>}</span></div>)}
 </div></section>
}
