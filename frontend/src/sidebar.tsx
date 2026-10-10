import {Icon} from './components'
import {Avatar,type Preferences} from './profile'

export function Sidebar({profile,isMeters,isProfile,onToggle,onLogout}:{profile:Preferences;isMeters:boolean;isProfile:boolean;onToggle:()=>void;onLogout:()=>void}){
 const links=[{href:'#/',title:'Inicio',icon:'grid',active:!isMeters&&!isProfile},{href:'#/meters',title:'Medidores',icon:'meter',active:isMeters},{href:'#/profile',title:'Mi perfil',icon:'profile',active:isProfile}]
 return <aside className="sidebar design-sidebar">
  <div className="sidebar-brand">
   <a className="sidebar-logo" href="#/" aria-label="energyIQ, inicio"><span className="sidebar-logo-symbol" aria-hidden="true">ϟ</span><span className="sidebar-wordmark">energy<span>IQ</span></span></a>
   <button className="sidebar-toggle icon-button" aria-label={profile.compact?'Expandir sidebar':'Contraer sidebar'} aria-expanded={!profile.compact} onClick={onToggle}><Icon name="chevron" size={13}/></button>
  </div>
  <div className="workspace-label"><strong>Operación demo</strong><small>12 medidores · datos demo</small></div>
  <nav aria-label="Navegación principal">{links.map(link=><a key={link.href} href={link.href} className={link.active?'active':''} aria-current={link.active?'page':undefined} title={link.title} aria-label={link.title}><Icon name={link.icon}/><span>{link.title}</span></a>)}</nav>
  <div className="sidebar-bottom"><a className="account" href="#/profile" title="Editar perfil"><Avatar profile={profile}/><div><strong>{profile.name}</strong><small>{profile.role}</small></div></a><button className="text-button logout" title="Cerrar sesión" aria-label="Cerrar sesión" onClick={onLogout}><Icon name="logout"/><span>Cerrar sesión</span></button></div>
 </aside>
}
