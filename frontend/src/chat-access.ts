import {useEffect,useState} from 'react'

// Read only viewport geometry: scrolling never fetches data or calls the assistant.
export function useChatAccess(){
 const [access,setAccess]=useState({visible:false,bottom:22})
 useEffect(()=>{
  let frame=0
  const root=document.querySelector('.workspace')
  const measure=()=>{
   frame=0
   const visible=[...document.querySelectorAll<HTMLElement>('[data-sonnet-entry]')].some(el=>{
    const r=el.getBoundingClientRect()
    return r.width>0&&r.height>0&&r.top<innerHeight-16&&r.bottom>16&&r.left<innerWidth&&r.right>0
   })
   const bar=document.querySelector('.profile-savebar')?.getBoundingClientRect()
   const inset=innerWidth<=650?16:22
   const bottom=bar&&bar.top<innerHeight&&bar.bottom>0&&bar.right>innerWidth-260
    ?Math.max(inset,innerHeight-bar.top+12):inset
   setAccess(previous=>previous.visible===!visible&&previous.bottom===bottom?previous:{visible:!visible,bottom})
  }
  const schedule=()=>{if(!frame)frame=requestAnimationFrame(measure)}
  const mutations=new MutationObserver(schedule)
  if(root)mutations.observe(root,{childList:true,subtree:true})
  const resize=new ResizeObserver(schedule)
  if(root)resize.observe(root)
  window.addEventListener('scroll',schedule,{passive:true,capture:true})
  window.addEventListener('resize',schedule)
  schedule()
  return()=>{cancelAnimationFrame(frame);mutations.disconnect();resize.disconnect();window.removeEventListener('scroll',schedule,true);window.removeEventListener('resize',schedule)}
 },[])
 return access
}
