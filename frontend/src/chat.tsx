import {useEffect,useMemo,useRef,useState,type FormEvent} from 'react'
import {useQuery,useQueryClient} from '@tanstack/react-query'
import {api} from './api'
import {useChatAccess} from './chat-access'
import {Badge,ErrorBox,Icon} from './components'
import {answerSections,chatCards,type MeterChatCard} from './chat-presentation'
import type {Finding,Meter,Page} from './types'
type Source={id:string;label:string;meter_id?:string}
type Message={receivedAt?:number;role:'user'|'assistant';content:string;sources?:Source[];cached?:boolean;model?:string;cards?:MeterChatCard[];priority?:boolean}
type Reply={answer:string;sources:Source[];cached:boolean;model:string;created_at:string}
type Status={enabled:boolean;model:string;daily_limit:number;used_today:number}
export function Chat({meterId,onPendingChange}:{meterId:string;onPendingChange?:(pending:boolean)=>void}){
 const query=useQueryClient()
 const access=useChatAccess()
 const [open,setOpen]=useState(false),[draft,setDraft]=useState(''),[threads,setThreads]=useState<Record<string,Message[]>>({}),[pending,setPending]=useState(false),[error,setError]=useState<unknown>(null)
 const opener=useRef<HTMLElement|null>(null)
 const busy=useRef(false),input=useRef<HTMLTextAreaElement>(null),conversation=useRef<HTMLDivElement>(null)
 const scope=meterId||'plant',messages=threads[scope]??[]
 const status=useQuery({queryKey:['chat-status'],queryFn:()=>api<Status>('/ai/chat'),enabled:open})
 useEffect(()=>{onPendingChange?.(pending)},[pending,onPendingChange])
 function close(){setOpen(false);requestAnimationFrame(()=>{const target=opener.current?.isConnected?opener.current:document.querySelector<HTMLElement>('.chat-launcher');target?.focus({preventScroll:true})})}
 const sendRef=useRef<(question:string)=>Promise<void>>(async()=>{})
 useEffect(()=>{const show=(e:Event)=>{const detail=(e as CustomEvent<{message:string;send?:boolean}>).detail;opener.current=document.activeElement as HTMLElement;setOpen(true);setError(null);if(detail.send)void sendRef.current(detail.message);else setDraft(detail.message)};window.addEventListener('energy-chat',show);return()=>window.removeEventListener('energy-chat',show)},[])
 useEffect(()=>{setDraft('');setError(null)},[scope])
 useEffect(()=>{if(open)input.current?.focus()},[open,scope])
 useEffect(()=>{const container=conversation.current;if(!container)return;const last=container.querySelector('.chat-message:last-of-type');if(messages.at(-1)?.role==='assistant'&&last)container.scrollTop+=last.getBoundingClientRect().top-container.getBoundingClientRect().top;else container.scrollTop=container.scrollHeight},[messages.length,pending,open,scope])
 async function send(question:string){
  question=question.trim();if(busy.current||question.length<2)return
  if(status.data?.enabled===false){setError(new Error('El chat no está disponible en este momento.'));return}
  busy.current=true;const key=scope,selected=meterId
  const history=messages.slice(-6).map(m=>({role:m.role,content:m.content}))
  const bytes=(text:string)=>new TextEncoder().encode(text).length
  while(history.length&&(history.some(m=>bytes(m.content)>6000)||history.reduce((n,m)=>n+bytes(m.content),0)>12000))history.shift()
  setPending(true);setError(null);setDraft('');setThreads(t=>({...t,[key]:[...(t[key]??[]),{role:'user',content:question}]}))
  try{
   // Existing query cache supplies verified navigation and numerical cards, without model calls.
   const [meters,findings]=await Promise.all([
    query.fetchQuery({queryKey:['meters-overview'],queryFn:()=>api<Page<Meter>>('/meters?sort=severity&limit=100'),staleTime:60000}),
    query.fetchQuery({queryKey:['findings-overview'],queryFn:()=>api<Page<Finding>>('/anomalies?limit=100'),staleTime:60000})
   ])
   const r=await api<Reply>('/ai/chat',{message:question,meter_id:selected,history})
   const presentation=chatCards(question,r.answer,r.sources??[],meters.items,findings.items,selected)
   setThreads(t=>({...t,[key]:[...(t[key]??[]),{role:'assistant',receivedAt:Date.now(),content:r.answer,sources:r.sources.map(s=>({...s,meter_id:meters.items.some(m=>m.meter_id===s.meter_id)?s.meter_id:undefined})),cached:r.cached,model:r.model,...presentation}]}))
  }catch(err){setError(err);setDraft(question);setThreads(t=>({...t,[key]:(t[key]??[]).slice(0,-1)}))}
  finally{busy.current=false;setPending(false);void query.invalidateQueries({queryKey:['chat-status']})}
 }
 sendRef.current=send
 function submit(e:FormEvent){e.preventDefault();void send(draft)}
 const suggestions=meterId?[`¿Qué cambió en ${meterId}?`,'¿Qué significa su factor de potencia?','¿Qué debo revisar primero?']:['¿Qué medidor merece atención primero?','¿Qué cambió en el consumo?','Compara el consumo de M-109 y M-104.']
 return <>
  {access.visible&&!open&&<button className="chat-launcher" style={{bottom:access.bottom}} onClick={e=>{opener.current=e.currentTarget;setOpen(!open)}} aria-expanded={open} aria-label="Hablar con Sonnet"><Icon name="spark"/><span>Pregúntale a Sonnet</span></button>}
  {open&&<section className="chat-panel" role="region" aria-label="Conversación con Sonnet" onKeyDown={e=>{if(e.key==='Escape'){close()}}}>
   <header><div><span className="chat-avatar"><Icon name="spark"/></span><div><strong>Sonnet <span className="assistant-label">Asistente de energía</span></strong>{meterId&&<small>{meterId}</small>}</div></div><div><button className="text-button" disabled={pending} onClick={()=>{setThreads(t=>({...t,[scope]:[]}));setError(null)}} title="Nueva conversación" aria-label="Nueva conversación">↻</button><button className="text-button" onClick={close} aria-label="Cerrar chat">✕</button></div></header>
   <div className="chat-messages" ref={conversation}>
    {!messages.length&&<div className="chat-welcome"><span className="assistant-orb"><Icon name="spark" size={26}/></span><h3>De tus datos a la respuesta.</h3><p>Qué cambió, qué significa y dónde mirar primero.</p><div>{suggestions.map(q=><button className="ghost" key={q} disabled={pending||!status.data?.enabled} onClick={()=>void send(q)}>{q}<Icon name="arrow" size={15}/></button>)}</div><small>Elige una pregunta para consultarla directamente.</small></div>}
    {messages.map((m,i)=><article className={'chat-message '+m.role} key={scope+i}>
     <small>{m.role==='user'?'Tú':'Sonnet'}</small>
     {m.role==='assistant'?<>
      {!!m.cards?.length&&<div className="chat-meter-results"><div className="chat-results-label">{m.priority?'Revisa estos medidores primero':'Datos clave del medidor'}</div>{m.cards.map(card=><a className={'chat-meter-card '+card.status.toLowerCase()} href={'#/meter/'+encodeURIComponent(card.id)} key={card.id} onClick={()=>setOpen(false)} aria-label={'Abrir '+card.id}><div><strong>{card.priority?`${card.priority}. `:''}{card.id}</strong><Badge value={card.status}/><Icon name="arrow" size={15}/></div><p>{card.title}</p><div className="chat-meter-value"><span>{card.metric}</span><b>{card.value}</b></div><small>Referencia {card.reference} · {card.period}</small><span className="chat-card-link">Ver medidor y análisis ↗</span></a>)}</div>}
      <Answer message={m}/>
     </>:<p>{m.content}</p>}
    </article>)}
    {pending&&<p className="chat-thinking" role="status"><span className="spinner"/>Sonnet está revisando el contexto…</p>}
    <ErrorBox error={error??status.error??(status.data&&!status.data.enabled?new Error("El chat no está disponible en este momento."):null)}/>
   </div>
   <form onSubmit={submit}><label className="sr-only" htmlFor="chat-question">Pregunta sobre energía</label><textarea id="chat-question" ref={input} maxLength={2000} value={draft} onChange={e=>setDraft(e.target.value)} placeholder={meterId?'Pregunta sobre '+meterId+'…':'Pregunta sobre tu consumo…'} onKeyDown={e=>{if(e.key==='Enter'&&!e.shiftKey){e.preventDefault();e.currentTarget.form?.requestSubmit()}}}/><button aria-label="Enviar pregunta" disabled={pending||draft.trim().length<2||!status.data?.enabled}><Icon name="arrow" size={17}/></button></form>

  </section>}
 </>
}
function InlineText({text}:{text:string}){return <>{text.split(/(\*\*[^*]+\*\*)/g).map((part,i)=>part.startsWith('**')&&part.endsWith('**')?<strong key={i}>{part.slice(2,-2)}</strong>:part)}</>}
function BodyText({text}:{text:string}){return <>{text.split(/\n\s*\n/).map((paragraph,i)=>{const lines=paragraph.split('\n');return lines.every(line=>/^\s*[-•]\s/.test(line))?<ul key={i}>{lines.map((line,j)=><li key={j}><InlineText text={line.replace(/^\s*[-•]\s/,'')}/></li>)}</ul>:<p key={i}><InlineText text={paragraph}/></p>})}</>}
// The animation reveals received text locally. It never creates another model request.
function Answer({message}:{message:Message}){
 const [expanded,setExpanded]=useState(false)
 const preview=message.content.length>650?message.content.slice(0,650).replace(/\s+\S*$/,'')+'…':message.content
 const sections=answerSections(message.content),structured=sections.some(s=>s.kind!=='prose')
 const text=expanded||structured?message.content:preview
 const frames=useMemo(()=>{
  let end=0,at=160
  return (text.match(/\S+\s*/g)??[]).map(word=>{
   end+=word.length;at+=Math.min(130,Math.max(65,word.trim().length*12))
   const frame={end,at};at+=/[.!?…]\s*$/.test(word)?170:/[,;:]\s*$/.test(word)?85:0
   if(word.includes('\n'))at+=120
   return frame
  })
 },[text])
 const duration=frames.at(-1)?.at??0
 const [elapsed,setElapsed]=useState(()=>window.matchMedia('(prefers-reduced-motion: reduce)').matches?Infinity:Date.now()-(message.receivedAt??0))
 useEffect(()=>{
  const motion=window.matchMedia('(prefers-reduced-motion: reduce)');const finish=()=>{if(motion.matches)setElapsed(Infinity)};motion.addEventListener('change',finish)
  const timer=window.setInterval(()=>{const next=Date.now()-(message.receivedAt??0);setElapsed(previous=>Math.max(previous,next));if(next>=duration)window.clearInterval(timer)},40)
  return()=>{window.clearInterval(timer);motion.removeEventListener('change',finish)}
 },[message.receivedAt,duration])
 const typing=elapsed<duration
 const visible=typing?text.slice(0,frames.filter(frame=>frame.at<=elapsed).at(-1)?.end??0):text
 return <><div className={'chat-answer '+(typing?'is-typing':'')} aria-hidden={typing||undefined}>
  {answerSections(visible).map((section,i)=><section className={'answer-section '+section.kind} key={i}>{section.title&&<h4><Icon name={section.kind==='recommendation'?'arrow':section.kind==='facts'?'activity':'help'} size={14}/>{section.title}</h4>}{!typing&&structured&&section.body.length>750?<LongSection text={section.body}/>:<BodyText text={section.body}/>}</section>)}
 </div>{typing?<button className="answer-toggle" onClick={()=>setElapsed(Infinity)}>Mostrar ahora</button>:!structured&&message.content.length>650?<button className="answer-toggle" aria-expanded={expanded} onClick={()=>{setElapsed(Infinity);setExpanded(!expanded)}}>{expanded?'Ver menos':'Leer respuesta completa'}</button>:null}<span className="sr-only" role="status">{typing?'Sonnet está escribiendo.':'Respuesta de Sonnet disponible.'}</span></>
}
function LongSection({text}:{text:string}){const [open,setOpen]=useState(false);return <><BodyText text={open?text:text.slice(0,650).replace(/\s+\S*$/,'')+'…'}/><button className="answer-toggle" aria-expanded={open} onClick={()=>setOpen(!open)}>{open?'Contraer bloque':'Ampliar bloque'}</button></>}
