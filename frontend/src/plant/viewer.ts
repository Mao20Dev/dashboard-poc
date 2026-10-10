import * as T from 'three'
import {GLTFLoader} from 'three/addons/loaders/GLTFLoader.js'
import {OrbitControls} from 'three/addons/controls/OrbitControls.js'
import anchors from './anchors.json'
import volumes from './occluders.json'

// Baked Blender illumination: no runtime lights, shadows or postprocessing.
export function createPlantViewer(root:HTMLElement, host:HTMLElement, onReady:()=>void, onError:()=>void, close:()=>void){
 const renderer=new T.WebGLRenderer({alpha:true,antialias:true,powerPreference:'low-power'})
 renderer.outputColorSpace=T.SRGBColorSpace;renderer.toneMapping=T.NoToneMapping;renderer.setClearColor(0,0)
 host.append(renderer.domElement)
 renderer.domElement.setAttribute('aria-label','Planta 3D. Arrastra para girar y usa los botones para acercar.')
 const scene=new T.Scene(),camera=new T.OrthographicCamera(-33,33,21,-21,.1,200)
 camera.position.set(45,45,59);camera.zoom=1.12
 const controls=new OrbitControls(camera,renderer.domElement)
 controls.target.set(0,2.2,0);controls.enableDamping=true;controls.dampingFactor=.12;controls.enablePan=false;controls.rotateSpeed=.45;controls.zoomSpeed=.55;controls.minZoom=.85;controls.maxZoom=1.8;controls.minPolarAngle=.5;controls.maxPolarAngle=1.2;controls.update();controls.saveState()
 // Wheel remains page scroll; explicit buttons zoom without trapping scrolling.
 controls.enableZoom=false
 const proxyMaterial=new T.MeshBasicMaterial(),occluders=volumes.map(v=>{const [x,y,z]=v.size;const m=new T.Mesh(v.shape==='cylinder'?new T.CylinderGeometry(x/2,x/2,y,16):new T.BoxGeometry(x,y,z),proxyMaterial);m.position.fromArray(v.position);m.updateMatrixWorld();return m})
 const points=anchors.map(a=>({...a,world:new T.Vector3(a.position[0],a.position[2],-a.position[1])}))
 const ray=new T.Raycaster(),p=new T.Vector3(),pointer=new T.Vector2()
 const textures=new Set<T.Texture>(),materials=new Set<T.Material>(),geometries=new Set<T.BufferGeometry>()
 const blend={value:0},nightUniform:{value:T.Texture|null}={value:null}
 let disposed=false,pending=0,ready=false,notified=false,visible=true,last=0,signature='',nightReady=false,zoomTarget=1.12,zooming=false
 const dark=()=>document.documentElement.dataset.theme==='dark'?1:0
 function project(){
  camera.updateMatrixWorld()
  const next=camera.matrixWorld.elements.join(',')+camera.zoom+root.clientWidth+root.clientHeight
  if(signature===next)return;signature=next
  const projected=new Map<string,{x:number;y:number;hidden:boolean}>()
  for(const a of points){
   p.copy(a.world).project(camera);pointer.set(p.x,p.y);ray.setFromCamera(pointer,camera)
   const hit=ray.intersectObjects(occluders,false)[0]
   const hidden=Math.abs(p.x)>1||Math.abs(p.y)>1||p.z>1||Boolean(hit&&hit.distance<a.world.distanceTo(ray.ray.origin)-.7)
   const x=(p.x*.5+.5)*100,y=(-p.y*.5+.5)*100
   projected.set(a.id,{x,y,hidden})
   const el=root.querySelector<HTMLElement>(`[data-plant-id="${a.id}"]`)
   if(el){el.style.left=x+'%';el.style.top=y+'%';el.style.visibility=hidden?'hidden':'visible';el.tabIndex=hidden?-1:0;el.setAttribute('aria-hidden',String(hidden))}
  }
  root.querySelectorAll<SVGPathElement>('[data-from]').forEach(path=>{const a=projected.get(path.dataset.from!),b=projected.get(path.dataset.to!);if(!a||!b)return;path.setAttribute('d',`M ${a.x*18} ${a.y*11.6} Q ${(a.x+b.x)*9} ${(a.y+b.y)*5.8+18} ${b.x*18} ${b.y*11.6}`);path.style.visibility=a.hidden||b.hidden?'hidden':'visible'})
 }
 function request(){if(!disposed&&!pending&&visible&&!document.hidden)pending=requestAnimationFrame(draw)}
 function draw(now:number){
  pending=0;if(disposed||!visible||document.hidden)return
  const dt=Math.min((now-last)/1000||.016,.06);last=now
  if(zooming){camera.zoom=T.MathUtils.damp(camera.zoom,zoomTarget,10,dt);if(Math.abs(camera.zoom-zoomTarget)<.001){camera.zoom=zoomTarget;zooming=false}camera.updateProjectionMatrix()}
  const moved=controls.update(),target=dark()
  if(nightReady){blend.value=T.MathUtils.damp(blend.value,target,8,dt);if(Math.abs(blend.value-target)<.002)blend.value=target}
  renderer.render(scene,camera);if(ready){project();if(!notified){notified=true;onReady()}}
  root.dataset.frames=String(Number(root.dataset.frames||0)+1)
  if(moved||zooming||(nightReady&&blend.value!==target))request()
 }
 function resize(){const w=root.clientWidth,h=root.clientHeight;if(!w||!h)return;const aspect=w/h,half=Math.max(21,33/aspect);camera.left=-half*aspect;camera.right=half*aspect;camera.top=half;camera.bottom=-half;camera.updateProjectionMatrix();renderer.setPixelRatio(Math.min(devicePixelRatio,1.25,Math.sqrt(1800000/(w*h))));renderer.setSize(w,h);signature='';close();request()}
 const ro=new ResizeObserver(resize);ro.observe(root)
 const mo=new MutationObserver(request);mo.observe(document.documentElement,{attributes:true,attributeFilter:['data-theme']})
 function visibility(){root.classList.toggle('eq-plant-paused',!visible||document.hidden);request()}
 const io=new IntersectionObserver(([entry])=>{visible=entry.isIntersecting;visibility()});io.observe(root)
 document.addEventListener('visibilitychange',visibility)
 controls.addEventListener('change',request);controls.addEventListener('start',close)
 function disposeModel(model:T.Object3D){model.traverse(o=>{if(o instanceof T.Mesh){o.geometry.dispose();const mats=Array.isArray(o.material)?o.material:[o.material];mats.forEach(m=>{const map=(m as T.MeshBasicMaterial).map;if(map)map.dispose();m.dispose()})}})}
 async function load(){
  const gltf=await new GLTFLoader().loadAsync('/models/plant/campus-baked.glb')
  if(disposed){disposeModel(gltf.scene);return}
  gltf.scene.traverse(o=>{if(o instanceof T.Mesh){geometries.add(o.geometry);const original=o.material as T.MeshStandardMaterial;const day=original.map??original.emissiveMap;if(!day)throw new Error('Missing baked texture');textures.add(day);day.colorSpace=T.SRGBColorSpace;day.anisotropy=Math.min(4,renderer.capabilities.getMaxAnisotropy());nightUniform.value=day
   const mat=new T.MeshBasicMaterial({map:day,toneMapped:false});materials.add(mat)
   mat.onBeforeCompile=s=>{s.uniforms.uNight=nightUniform;s.uniforms.uTheme=blend;s.fragmentShader='uniform sampler2D uNight; uniform float uTheme;\n'+s.fragmentShader;s.fragmentShader=s.fragmentShader.replace('#include <map_fragment>','vec4 atlasColor=texture2D(map,vMapUv); if(uTheme>0.0) atlasColor=mix(atlasColor,texture2D(uNight,vMapUv),uTheme); diffuseColor*=atlasColor;')}
   mat.customProgramCacheKey=()=> 'energyiq-baked-day-night';o.material=mat;original.dispose();o.matrixAutoUpdate=false
  }})
  scene.add(gltf.scene);scene.updateMatrixWorld(true);await renderer.compileAsync(scene,camera)
  if(disposed)return;ready=true;signature='';request()
  try{const night=await new T.TextureLoader().loadAsync('/models/plant/campus-night-atlas.png');if(disposed){night.dispose();return}textures.add(night);night.colorSpace=T.SRGBColorSpace;night.flipY=false;night.anisotropy=Math.min(4,renderer.capabilities.getMaxAnisotropy());nightUniform.value=night;nightReady=true;request()}catch{ /* Day atlas remains usable if night asset fails. */ }
 }
 resize();void load().catch(()=>{if(!disposed)onError()})
 return {
  refresh(){signature='';request()},
  zoom(step:number){zoomTarget=T.MathUtils.clamp(camera.zoom+step,.85,1.8);zooming=true;close();request()},
  reset(){zooming=false;controls.reset();signature='';close();request()},
  dispose(){disposed=true;cancelAnimationFrame(pending);ro.disconnect();mo.disconnect();io.disconnect();document.removeEventListener('visibilitychange',visibility);controls.dispose();textures.forEach(t=>t.dispose());materials.forEach(m=>m.dispose());geometries.forEach(g=>g.dispose());occluders.forEach(o=>o.geometry.dispose());proxyMaterial.dispose();renderer.dispose();renderer.forceContextLoss();renderer.domElement.remove()}
 }
}
