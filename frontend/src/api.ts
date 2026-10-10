export class APIError extends Error {constructor(public status:number,message:string){super(message)}}
export async function api<T>(path:string,body?:unknown,method?:string):Promise<T>{
 const response=await fetch('/api/v1'+path,{credentials:'same-origin',method:method??(body===undefined?'GET':'POST'),headers:body===undefined?{}:{'Content-Type':'application/json'},body:body===undefined?undefined:JSON.stringify(body)})
 const data=await response.json()
 if(!response.ok)throw new APIError(response.status,data.title??'No fue posible completar la operación.')
 return data as T
}
export const num=(n:number,digits=2)=>new Intl.NumberFormat('es-CO',{maximumFractionDigits:digits}).format(n)
export const date=(s:string)=>new Date(s).toLocaleString('es-CO',{timeZone:'America/Bogota',dateStyle:'medium',timeStyle:'short'})
export const labels:Record<string,string>={REAL_ANOMALY:'Sin explicación operativa',DATA_QUALITY:'Calidad de medición',EXPLAINABLE_ANOMALY:'Cambio explicable',FALSE_POSITIVE:'Parada explicada',HIGH:'Alta',MEDIUM:'Media',LOW:'Baja',OPEN:'Abierta',INVESTIGATING:'En investigación',RESOLVED:'Resuelta',DISMISSED:'Descartada',NORMAL:'Normal',CRITICAL:'Crítico',ALERT:'Alerta',NOT_ANALYZED:'Sin analizar',INSUFFICIENT_DATA:'Datos insuficientes',PENDING:'En cola',RUNNING:'En curso',COMPLETED:'Completado',FAILED:'Falló',QUEUED:'En cola',READINGS:'Leyendo datos',DETECTION:'Detectando señales',EXPLANATION:'Preparando investigación',PERSISTENCE:'Guardando evidencia',NOTE:'Nota',INVESTIGATE:'Investigar',RESOLVE:'Resolver',DISMISS:'Descartar',REOPEN:'Reabrir',consumption_kwh:'Consumo',voltage_v:'Voltaje',current_a:'Corriente',power_factor:'Factor de potencia'}
export const label=(s:string)=>labels[s]??s
