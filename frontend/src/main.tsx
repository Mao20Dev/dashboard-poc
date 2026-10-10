import { StrictMode, useEffect, useState } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, APIError } from './api'
import type { Run, Provider, Dashboard } from './types'
import { ErrorBox, Icon, Loading } from './components'
import { Overview } from './overview'
import {PeriodRange} from './home'
import {AnalysisStatus,analysisBusy} from './analysis'
import { Chat } from './chat'
import { LoginBee } from './login-bee'
import { Avatar, Profile, loadPreferences, type Preferences } from './profile'
import './style.css'
import {Sidebar} from './sidebar'
import './sidebar.css'
import {Select} from './select'
import {useTheme} from './theme'
import './motion.css'
const client=new QueryClient({defaultOptions:{queries:{retry:false,refetchOnWindowFocus:false}}})
const demo={email:'uribe@energyiq.test',password:'EnergyDemo2026!'}
function Brand(){return <a className="brand" href="#/" aria-label="energyIQ, inicio"><span className="brand-icon"><Icon size={19}/></span>energy<span>IQ</span></a>}
function ThemeSwitch(){
 const [theme,setTheme]=useTheme()
 const toggle=()=>setTheme(theme==='dark'?'light':'dark')
 return <button className="theme-switch ghost" onClick={toggle} aria-label={theme==='dark'?'Cambiar a tema claro':'Cambiar a tema oscuro'} title={theme==='dark'?'Tema claro':'Tema oscuro'}><span aria-hidden="true">{theme==='dark'?'☀':'☾'}</span><span>{theme==='dark'?'Claro':'Oscuro'}</span></button>
}
function Login(){
 const query=useQueryClient()
 const m=useMutation({mutationFn:(credentials:typeof demo)=>api('/auth/login',credentials),onSuccess:()=>query.invalidateQueries({queryKey:['me']})})
 return <div className="login-shell"><div className="login-top"><Brand/><ThemeSwitch/></div><LoginBee/><main className="login"><div className="login-content"><span className="eyebrow">TU ENERGÍA, EN CONTEXTO</span><h1>Bienvenido a una visión más clara.</h1><p>Entiende tu consumo, identifica lo que cambió y encuentra lo que merece tu atención.</p><button className="demo-button" disabled={m.isPending} onClick={()=>m.mutate(demo)}><span>{m.isPending?'Entrando…':'Entrar a la demo'}</span>{m.isPending?<span className="spinner"/>:<Icon name="arrow"/>}</button><ErrorBox error={m.error}/></div></main></div>
}
function Workspace({email}:{email:string}){
 const query=useQueryClient();const [route,setRoute]=useState(location.hash.slice(1)||'/');const [menu,setMenu]=useState(false);const [profile,setProfile]=useState(()=>loadPreferences(email))
 useEffect(()=>{const change=()=>{setRoute(location.hash.slice(1)||'/');setMenu(false);window.scrollTo(0,0)};window.addEventListener('hashchange',change);return()=>window.removeEventListener('hashchange',change)},[])
 const [chatPending,setChatPending]=useState(false)
 const dashboard=useQuery({queryKey:['dashboard'],queryFn:()=>api<Dashboard>('/dashboard/summary')})
 const providers=useQuery({queryKey:['providers'],queryFn:()=>api<{items:Provider[];default:string;allow_new_requests:boolean}>('/ai/providers')})
 const last=useQuery({queryKey:['latest'],queryFn:async()=>{try{return await api<Run>('/ai/analysis/latest')}catch(e){if(e instanceof APIError&&e.status===404)return null;throw e}}})
 useEffect(()=>{if(last.data?.status==='COMPLETED')void query.invalidateQueries({predicate:q=>!['me','latest','providers','analysis-progress'].includes(String(q.queryKey[0]))})},[last.data?.id,last.data?.status,query])
 const run=useMutation({mutationFn:(id:string)=>api<Run>('/ai/analyze',{provider:id,cached_only:id!=='rules'}),onSuccess:data=>query.setQueryData(['latest'],data)})
 const progress=useQuery({queryKey:['analysis-progress',last.data?.id],enabled:analysisBusy(last.data),queryFn:()=>api<Run>('/ai/analysis/'+encodeURIComponent(last.data!.id)),refetchInterval:q=>analysisBusy(q.state.data)?800:false})
 useEffect(()=>{if(progress.data&&progress.data.id===last.data?.id)query.setQueryData(['latest'],progress.data)},[progress.data,last.data?.id,query])
 const busy=run.isPending||analysisBusy(last.data)
 const controls=<div className="ai-controls"><Select label="Proveedor de IA" heading="Modelo de análisis" icon={<Icon name="spark" size={16}/>} value={last.data?.provider||providers.data?.default||'rules'} disabled={busy||!providers.data} onChange={id=>run.mutate(id)} options={(providers.data?.items??[]).map(p=>({value:p.id,label:p.name,disabled:!p.configured,description:p.id==='claude'?'Interpretación con Claude':p.id==='openai'?'Interpretación con OpenAI':'Detección y evidencia del motor local'}))}/><button disabled={busy||!providers.data} onClick={()=>run.mutate(last.data?.provider||providers.data?.default||'rules')}>{busy?<span className="spinner"/>:<Icon name="spark" size={14}/>}Analizar con IA</button></div>
 const analysisStatus=<><AnalysisStatus run={last.data??null} requesting={run.isPending}/>{progress.error&&<div className="analysis-status"><ErrorBox error={progress.error}/><button className="ghost" onClick={()=>progress.refetch()}>Actualizar estado</button></div>}</>
 const logout=useMutation({mutationFn:()=>api('/auth/logout',{}),onSuccess:()=>{query.clear();window.location.assign('/')}})
 const meterDetailId=route.startsWith('/meter/')?decodeURIComponent(route.split('/')[2]):new URL(route,'http://local').searchParams.get('meter');const isProfile=route==='/profile';const isMeters=route.startsWith('/meters')||route.startsWith('/meter/')||route.startsWith('/incident/')
 function saveProfile(next:Preferences){localStorage.setItem('energyiq.profile.'+email,JSON.stringify(next));setProfile(next)}
 return <div className={'app-shell '+(profile.compact?'compact ':'')+(menu?'menu-open':'')}><button className="menu-backdrop" aria-label="Cerrar menú" onClick={()=>setMenu(false)}/><Sidebar profile={profile} isMeters={isMeters} isProfile={isProfile} onToggle={()=>saveProfile({...profile,compact:!profile.compact})} onLogout={()=>logout.mutate()}/><main className="workspace"><header className="workspace-header"><div className="header-location"><button className="icon-button mobile-menu" aria-label="Abrir menú" onClick={()=>setMenu(true)}><Icon name="menu"/></button><span>Mi espacio</span><span>/</span><strong>{isProfile?'Mi perfil':isMeters?<><a href="#/meters">Medidores</a>{meterDetailId&&<> / {meterDetailId}</>}</>:'Inicio'}</strong></div><div className="header-actions">{!isProfile&&<PeriodRange data={dashboard.data}/>}<ThemeSwitch/><a href="#/profile" aria-label="Editar mi perfil"><Avatar profile={profile}/></a></div></header><ErrorBox error={run.error??last.error??providers.error??logout.error}/><div className="route-stage" key={route}>{isProfile?<Profile email={email} profile={profile} dashboard={dashboard.data} onLogout={()=>logout.mutate()} logoutPending={logout.isPending} onSave={next=>saveProfile({...next,compact:profile.compact})}/>:<Overview chatPending={chatPending} route={route} name={profile.name} technical={profile.technical} run={last.data??null} controls={controls} analysisStatus={analysisStatus}/>}</div><Chat onPendingChange={setChatPending} meterId={route.startsWith('/meter/')?decodeURIComponent(route.split('/')[2]):new URL(route,'http://local').searchParams.get('meter')||''}/><footer><span>energyIQ · Lecturas con contexto</span><span>Hora de Colombia</span></footer></main></div>
}
function App(){const me=useQuery({queryKey:['me'],queryFn:()=>api<{email:string}>('/auth/me')});if(me.isPending)return <Loading/>;if(me.error instanceof APIError&&me.error.status===401)return <Login/>;if(me.error||!me.data)return <main><ErrorBox error={me.error}/><button onClick={()=>me.refetch()}>Reintentar conexión</button></main>;return <Workspace email={me.data.email}/>}
createRoot(document.getElementById('root')!).render(<StrictMode><QueryClientProvider client={client}><App/></QueryClientProvider></StrictMode>)

