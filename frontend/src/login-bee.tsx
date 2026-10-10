import {useEffect,useRef,useState} from 'react'
export function LoginBee({variant='login'}:{variant?:'login'|'home'}){
 const host=useRef<HTMLDivElement>(null)
 const [state,setState]=useState<'loading'|'ready'|'fallback'>('loading')
 const controller=useRef<{dispose:()=>void;setPaused:(value:boolean)=>void}|undefined>(undefined)
 useEffect(()=>{
  let disposed=false
  import('./bee-scene').then(({mountBee})=>{
   if(disposed||!host.current)return
   controller.current=mountBee(host.current,setState,variant)
  }).catch(()=>{if(!disposed)setState('fallback')})
  const motion=window.matchMedia('(prefers-reduced-motion: reduce)')
  const change=()=>{controller.current?.setPaused(motion.matches)}
  motion.addEventListener('change',change)
  return()=>{disposed=true;motion.removeEventListener('change',change);controller.current?.dispose();controller.current=undefined}
 },[variant])
 return <aside className={variant==='home'?'home-bee':'bee-stage login-bee'} aria-label="Abeja robótica interactiva" data-scene="bee" data-state={state}>
  <div className="bee-viewport" ref={host} aria-hidden="true"/>
  {state!=='ready'&&<div className="bee-static" aria-hidden="true"><svg viewBox="0 0 260 200" fill="none"><ellipse cx="86" cy="72" rx="48" ry="22" transform="rotate(-35 86 72)"/><ellipse cx="174" cy="72" rx="48" ry="22" transform="rotate(35 174 72)"/><path d="M103 78c-28 49-11 93 27 104 38-11 55-55 27-104M102 117h56m-50 28h44M99 103l-31 17m36 22-23 23m80-62 31 17m-36 22 23 23"/><circle cx="130" cy="68" r="30"/><path d="m116 41-12-20m40 20 12-20"/><circle cx="119" cy="65" r="4"/><circle cx="141" cy="65" r="4"/></svg></div>}
  {variant==='login'&&<div className="scene-caption"><h2>La energía cobra vida.</h2></div>}
 </aside>
}
