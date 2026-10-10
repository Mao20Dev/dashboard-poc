export function PageSkeleton({kind='home'}:{kind?:'home'|'detail'|'table'}){
 return <div className={'page-skeleton skeleton-'+kind} role="status" aria-label={kind==='table'?'Cargando medidores':'Cargando información'} aria-busy="true">
  <span className="sr-only">Cargando información…</span>
  <div aria-hidden="true">
   {kind==='table'?Array.from({length:12},(_,i)=><div className="skeleton-row" key={i}>{Array.from({length:6},(_,j)=><span className="skeleton-shape" key={j}/>)}</div>):<>
    <div className="skeleton-hero"><span className="skeleton-shape skeleton-heading"/><span className="skeleton-shape skeleton-line"/><span className="skeleton-shape skeleton-line short"/></div>
    <div className="skeleton-cards">{[0,1,2].map(i=><div className="skeleton-card" key={i}><span className="skeleton-shape skeleton-heading"/><span className="skeleton-shape skeleton-line"/><span className="skeleton-shape skeleton-line short"/></div>)}</div>
   </>}
  </div>
 </div>
}
