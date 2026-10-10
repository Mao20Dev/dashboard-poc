import {useSyncExternalStore} from 'react'
type Theme='light'|'dark'
const current=():Theme=>document.documentElement.dataset.theme==='dark'?'dark':'light'
function subscribe(notify:()=>void){
 const observer=new MutationObserver(notify)
 observer.observe(document.documentElement,{attributes:true,attributeFilter:['data-theme']})
 return ()=>observer.disconnect()
}
export function useTheme(){
 const theme=useSyncExternalStore(subscribe,current)
 function setTheme(next:Theme){document.documentElement.dataset.theme=next;try{localStorage.setItem('energyiq.theme',next)}catch{}}
 return [theme,setTheme] as const
}
