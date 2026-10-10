import {type ReactNode} from 'react'
import {useQuery} from '@tanstack/react-query'
import {api} from './api'
import {ErrorBox} from './components'
import {Meters} from './meters'
import {MeterPage} from './meter-page'
import {Home} from './home'
import {PageSkeleton} from './skeleton'
import type {Dashboard,Finding,Meter,Page,Run} from './types'

export function Overview({route,name,technical,run,controls,analysisStatus,chatPending}:{route:string;name:string;technical:boolean;run:Run|null;controls:ReactNode;analysisStatus:ReactNode;chatPending:boolean}){
 const dashboard=useQuery({queryKey:['dashboard'],queryFn:()=>api<Dashboard>('/dashboard/summary')})
 const meters=useQuery({queryKey:['meters-overview'],queryFn:()=>api<Page<Meter>>('/meters?sort=severity&limit=100')})
 const findings=useQuery({queryKey:['findings-overview'],queryFn:()=>api<Page<Finding>>('/anomalies?limit=100')})
 const isMeters=route.startsWith('/meters')||route.startsWith('/meter/')||route.startsWith('/incident/')
 const url=new URL(route,'http://local');const legacy=route.startsWith('/incident/')?findings.data?.items.find(f=>f.id===route.split('/')[2])?.meter_id:undefined
 const selected=url.searchParams.get('meter')||(route.startsWith('/meter/')?decodeURIComponent(route.split('/')[2]):legacy)
 const d=dashboard.data;const all=meters.data?.items??[];const fs=findings.data?.items??[]
 if(selected)return <MeterPage key={selected} id={selected} technical={technical} run={run} controls={controls} analysisStatus={analysisStatus}/>
 if(!isMeters)return <><ErrorBox error={dashboard.error??meters.error??findings.error}/>{d&&!meters.isPending&&!findings.isPending?<Home name={name} data={d} meters={all} findings={fs} run={run} chatPending={chatPending} controls={controls} analysisStatus={analysisStatus}/>:(dashboard.isPending||meters.isPending||findings.isPending)?<PageSkeleton/>:null}</>
 return <Meters items={all} findings={fs} controls={controls} analysisStatus={analysisStatus} pending={meters.isPending} error={meters.error??findings.error}/>
}
