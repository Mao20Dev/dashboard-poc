import {useEffect,useId,useLayoutEffect,useRef,useState,type CSSProperties,type KeyboardEvent,type ReactNode} from 'react'
import {createPortal} from 'react-dom'
import {Icon} from './components'
import './select.css'

type Option={value:string;label:string;description?:string;disabled?:boolean}
export function Select({label,value,options,onChange,disabled=false,icon,heading}:{label:string;value:string;options:Option[];onChange:(value:string)=>void;disabled?:boolean;icon?:ReactNode;heading?:string}){
 const id=useId(),trigger=useRef<HTMLButtonElement>(null),menu=useRef<HTMLDivElement>(null)
 const [open,setOpen]=useState(false),[active,setActive]=useState(-1),[position,setPosition]=useState<CSSProperties>({})
 const search=useRef({text:'',at:0})
 const selected=options.find(o=>o.value===value)
 const enabled=options.map((o,i)=>o.disabled?-1:i).filter(i=>i>=0)
 function show(){if(disabled)return;setActive(options.findIndex(o=>o.value===value&&!o.disabled)>=0?options.findIndex(o=>o.value===value):enabled[0]??-1);setOpen(true)}
 function choose(index:number){const option=options[index];if(!option||option.disabled)return;setOpen(false);trigger.current?.focus({preventScroll:true});if(option.value!==value)onChange(option.value)}
 useEffect(()=>{if(disabled)setOpen(false)},[disabled])
 useLayoutEffect(()=>{
  if(!open)return
  const place=()=>{
   const r=trigger.current?.getBoundingClientRect();if(!r)return
   const width=Math.min(Math.max(r.width,heading?288:250),innerWidth-24)
   const below=innerHeight-r.bottom-20,above=r.top-20
   const upward=below<230&&above>below
   setPosition({position:'fixed',width,left:Math.max(12,Math.min(r.right-width,innerWidth-width-12)),maxHeight:Math.max(80,Math.min(360,upward?above:below)),...(upward?{bottom:innerHeight-r.top+8}:{top:r.bottom+8})})
  }
  place()
  const outside=(event:PointerEvent)=>{const node=event.target as Node;if(!trigger.current?.contains(node)&&!menu.current?.contains(node))setOpen(false)}
  document.addEventListener('pointerdown',outside)
  window.addEventListener('resize',place);window.addEventListener('scroll',place,true)
  return()=>{document.removeEventListener('pointerdown',outside);window.removeEventListener('resize',place);window.removeEventListener('scroll',place,true)}
 },[open,heading])
 useEffect(()=>{if(!open||active<0||!menu.current)return;const option=document.getElementById(id+'-'+active);if(!option)return;const box=menu.current.getBoundingClientRect(),item=option.getBoundingClientRect();if(item.bottom>box.bottom)menu.current.scrollTop+=item.bottom-box.bottom+7;else if(item.top<box.top)menu.current.scrollTop-=box.top-item.top+7},[open,active,id])
 function keyboard(event:KeyboardEvent<HTMLButtonElement>){
  if(event.key==='Escape'){if(open){event.preventDefault();event.stopPropagation();setOpen(false)}return}
  if(event.key==='Tab'){setOpen(false);return}
  if(['ArrowDown','ArrowUp','Home','End','Enter',' '].includes(event.key)){
   event.preventDefault()
   if(!open){show();return}
   if(event.key==='Enter'||event.key===' '){choose(active);return}
   const index=enabled.indexOf(active)
   setActive(event.key==='Home'?enabled[0]:event.key==='End'?enabled.at(-1)??-1:enabled[(index+(event.key==='ArrowDown'?1:-1)+enabled.length)%enabled.length]??-1)
  }else if(event.key.length===1&&!event.ctrlKey&&!event.metaKey&&!event.altKey){
   event.preventDefault();const now=Date.now();search.current={text:(now-search.current.at<700?search.current.text:'')+event.key.toLocaleLowerCase(),at:now}
   if(!open)show()
   const match=enabled.find(i=>options[i].label.toLocaleLowerCase().startsWith(search.current.text));if(match!==undefined)setActive(match)
  }
 }
 return <div className="soft-select"><button ref={trigger} type="button" className="soft-select-trigger" role="combobox" aria-label={label} aria-expanded={open} aria-haspopup="listbox" aria-controls={open?id:undefined} aria-activedescendant={open&&active>=0?id+'-'+active:undefined} disabled={disabled} onKeyDown={keyboard} onBlur={()=>setOpen(false)} onClick={()=>open?setOpen(false):show()}>
  {icon&&<span className="soft-select-icon">{icon}</span>}<span className="soft-select-value">{selected?.label??'Seleccionar'}</span><span className={'soft-select-chevron '+(open?'is-open':'')}><Icon name="chevron" size={14}/></span>
 </button>{open&&createPortal(<div className="soft-select-menu" ref={menu} style={position} onPointerDown={e=>e.preventDefault()}>
  {heading&&<div className="soft-select-heading">{heading}</div>}
  <ul id={id} role="listbox" aria-label={label}>{options.map((option,i)=><li id={id+'-'+i} key={option.value} role="option" aria-selected={option.value===value} aria-disabled={option.disabled||undefined} className={'soft-select-option '+(active===i?'is-active':'')} onPointerMove={()=>{if(!option.disabled)setActive(i)}} onClick={()=>choose(i)}>
   <span><strong>{option.label}</strong>{(option.description||option.disabled)&&<small>{option.disabled?'No disponible':option.description}</small>}</span><span className="soft-select-check">{option.value===value&&<Icon name="check" size={16}/>}</span>
  </li>)}</ul>
 </div>,document.body)}</div>
}
