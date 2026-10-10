import * as THREE from 'three'
import {GLTFLoader} from 'three/addons/loaders/GLTFLoader.js'
import {RoomEnvironment} from 'three/addons/environments/RoomEnvironment.js'
type State='loading'|'ready'|'fallback'
type Colors={attribute:THREE.BufferAttribute|THREE.InterleavedBufferAttribute;dark:Float32Array;light:Float32Array}
const palettes={dark:['#8fae97','#35423b','#d1ddd5','#9fb9ab'],light:['#35644b','#b3bdb4','#4c6357','#799e8a']}
// The user's bee-poc GLB retains its wing/leg pivots and per-vertex part colors.
// Recolor those part masks in memory, leaving the source model untouched.
export function mountBee(host:HTMLDivElement,onState:(state:State)=>void,variant:'login'|'home'='login'){
 let renderer:THREE.WebGLRenderer
 try{renderer=new THREE.WebGLRenderer({alpha:true,antialias:true,powerPreference:'low-power'})}
 catch{onState('fallback');return {dispose(){},setPaused(){}}}
 let disposed=false,loaded=false,paused=matchMedia('(prefers-reduced-motion: reduce)').matches,visible=true,frame=0,lastTime=0,elapsed=0,flapPhase=0
 let blend=document.documentElement.dataset.theme==='light'?1:0,targetBlend=blend,themeFrames=0
 const abort=new AbortController(),scene=new THREE.Scene(),camera=new THREE.PerspectiveCamera(variant==='home'?32:38,1,.1,50)
 camera.position.set(0,.1,4.6)
 renderer.setPixelRatio(Math.min(devicePixelRatio,1.5));renderer.setClearColor(0,0)
 renderer.outputColorSpace=THREE.SRGBColorSpace;renderer.toneMapping=THREE.ACESFilmicToneMapping;renderer.toneMappingExposure=1.08
 renderer.domElement.setAttribute('aria-hidden','true');host.append(renderer.domElement)
 const environment=new RoomEnvironment(),pmrem=new THREE.PMREMGenerator(renderer),environmentMap=pmrem.fromScene(environment,.04)
 scene.environment=environmentMap.texture;environment.dispose();pmrem.dispose()
 scene.add(new THREE.HemisphereLight(0xe6efe9,0x35453b,2))
 const key=new THREE.DirectionalLight(0xfff7e9,3);key.position.set(-2,4,5);scene.add(key)
 const rim=new THREE.DirectionalLight(0xb4cdbf,2);rim.position.set(3,2,-2);scene.add(rim)
 const position=new THREE.Group(),pose=new THREE.Group();scene.add(position);position.add(pose);pose.rotation.set(.12,100*Math.PI/180,0)
 let root:THREE.Object3D|undefined,wingL:THREE.Object3D|undefined,wingR:THREE.Object3D|undefined
 const legs:THREE.Object3D[]=[],colors:Colors[]=[],materials=new Set<THREE.Material>(),geometries=new Set<THREE.BufferGeometry>()
 const pointer=new THREE.Vector2(),retreat=new THREE.Vector3(),velocity=new THREE.Vector3(),target=new THREE.Vector3(),ray=new THREE.Raycaster(),center=new THREE.Vector3()
 const rest=new THREE.Vector3(),hit=new THREE.Vector3(),direction=new THREE.Vector3(),escapeDirection=new THREE.Vector3(0,1,0),acceleration=new THREE.Vector3(),plane=new THREE.Plane(new THREE.Vector3(0,0,1),0)
 let pointerInside=false,alert=0,alertOn=false,escapeStrength=0
 function applyColors(){
  for(const c of colors){for(let i=0;i<c.attribute.count;i++){const n=i*3;c.attribute.setXYZ(i,THREE.MathUtils.lerp(c.dark[n],c.light[n],blend),THREE.MathUtils.lerp(c.dark[n+1],c.light[n+1],blend),THREE.MathUtils.lerp(c.dark[n+2],c.light[n+2],blend))}c.attribute.needsUpdate=true}
  host.dataset.theme=targetBlend?'light':'dark'
 }
 function render(now:number){
  frame=0;if(disposed||!visible||document.hidden)return
  const dt=lastTime?Math.min((now-lastTime)/1000,.04):.016;lastTime=now
  if(themeFrames>0){blend=THREE.MathUtils.lerp(blend,targetBlend,1-Math.exp(-dt*9));themeFrames--;if(themeFrames===0)blend=targetBlend;applyColors()}
  if(loaded&&!paused){
   elapsed+=dt
   root?.getWorldPosition(center);ray.setFromCamera(pointer,camera)
   const scale=pose.scale.x,distance=ray.ray.distanceToPoint(center)
   // Hysteresis keeps the fast wingbeat steady while the bee retreats.
   alertOn=pointerInside&&distance<(alertOn?1.15:.85)*scale
   alert=THREE.MathUtils.damp(alert,alertOn?1:0,alertOn?4:2.5,dt)
   const proximity=pointerInside?THREE.MathUtils.smoothstep(1-distance/(1.3*scale),0,1):0
   escapeStrength=THREE.MathUtils.damp(escapeStrength,proximity,proximity>escapeStrength?6:3,dt)
   // Use the resting thorax, not the displaced bee, to avoid orbiting the cursor.
   // Normalize the escape vector: pointing at the center must still trigger flight.
   rest.set(0,.22*scale,0)
   if(proximity>0&&ray.ray.intersectPlane(plane,hit)){
    direction.subVectors(rest,hit)
    if(direction.lengthSq()<.0025)direction.set(0,1,0)
    direction.normalize()
    // Let the vector shorten during a reversal instead of snapping through 180°.
    escapeDirection.lerp(direction,1-Math.exp(-dt*5.5))
   }
   target.copy(escapeDirection).multiplyScalar(.65*scale*escapeStrength)
   // A responsive dodge with a slower return; critical damping avoids a snap or bounce.
   const omega=2*Math.PI*THREE.MathUtils.lerp(.65,1.3,escapeStrength)
   // Small integration steps keep the spring stable even on slower GPUs.
   const steps=Math.ceil(dt/.008),step=dt/steps
   for(let i=0;i<steps;i++){
    acceleration.copy(target).sub(retreat).multiplyScalar(omega*omega).addScaledVector(velocity,-2*omega)
    velocity.addScaledVector(acceleration,step);retreat.addScaledVector(velocity,step)
   }
   position.position.set(retreat.x+Math.sin(elapsed*.7)*.025,retreat.y+Math.sin(elapsed*.95)*.045,0)
   // Match bee-poc: 7 Hz at rest, 24 Hz on approach, with a wider wing stroke.
   flapPhase=(flapPhase+dt*(7+alert*17)*Math.PI*2)%(Math.PI*2)
   const swing=.12+Math.sin(flapPhase)*(.5+alert*.28),twist=Math.cos(flapPhase)*.18
   if(wingL){wingL.rotation.z=-swing;wingL.rotation.x=-twist}
   if(wingR){wingR.rotation.z=swing;wingR.rotation.x=twist}
   legs.forEach((leg,i)=>{leg.rotation.x=Math.sin(elapsed*2+i*2.1)*.055;leg.rotation.z=Math.sin(elapsed*1.3+i*2.1)*.035})
   if(root){root.rotation.x=Math.sin(elapsed*.7)*.025;root.rotation.z=Math.sin(elapsed*.55)*.02}
   host.dataset.reacting=alert>.03?'true':'false'
  }
  renderer.render(scene,camera)
  host.dataset.motion=paused?'paused':'active'
  if(loaded&&!paused||themeFrames>0)frame=requestAnimationFrame(render)
 }
 function schedule(){if(!disposed&&visible&&!document.hidden&&!frame){lastTime=0;frame=requestAnimationFrame(render)}}
 function resize(){const {width,height}=host.getBoundingClientRect();if(!width||!height)return;camera.aspect=width/height;camera.updateProjectionMatrix();renderer.setSize(width,height,false);pose.scale.setScalar(Math.min(1.3,Math.max(.65,camera.aspect*.98)));schedule()}
 const resizeObserver=new ResizeObserver(resize);resizeObserver.observe(host)
 const intersection=new IntersectionObserver(([entry])=>{visible=entry.isIntersecting;if(visible)schedule();else{cancelAnimationFrame(frame);frame=0}},{threshold:.05});intersection.observe(host)
 const themeObserver=new MutationObserver(()=>{targetBlend=document.documentElement.dataset.theme==='light'?1:0;themeFrames=matchMedia('(prefers-reduced-motion: reduce)').matches?1:45;schedule()});themeObserver.observe(document.documentElement,{attributes:true,attributeFilter:['data-theme']})
 function onPointer(event:PointerEvent){if(event.pointerType==='touch')return;const rect=host.getBoundingClientRect();pointer.set((event.clientX-rect.left)/rect.width*2-1,-(event.clientY-rect.top)/rect.height*2+1);pointerInside=true}
 function onLeave(){pointerInside=false}
 function onVisibility(){if(document.hidden){cancelAnimationFrame(frame);frame=0}else schedule()}
 function onLoss(event:Event){event.preventDefault();cancelAnimationFrame(frame);frame=0;loaded=false;onState('fallback')}
 host.addEventListener('pointermove',onPointer);host.addEventListener('pointerleave',onLeave);document.addEventListener('visibilitychange',onVisibility);renderer.domElement.addEventListener('webglcontextlost',onLoss)
 function disposeModel(model:THREE.Object3D){model.traverse(o=>{if(o instanceof THREE.Mesh){o.geometry.dispose();(Array.isArray(o.material)?o.material:[o.material]).forEach(m=>m.dispose())}})}
 fetch('/models/bee.glb',{signal:abort.signal}).then(response=>{if(!response.ok)throw new Error('Model unavailable');return response.arrayBuffer()}).then(data=>new GLTFLoader().parseAsync(data,'')).then(gltf=>{
  if(disposed){disposeModel(gltf.scene);return}
  root=gltf.scene.getObjectByName('bee_root')??gltf.scene
  gltf.scene.traverse(o=>{
   if(!(o instanceof THREE.Mesh))return
   geometries.add(o.geometry)
   const original=Array.isArray(o.material)?o.material:[o.material];original.forEach(m=>m.dispose())
   const wing=o.name==='bee_wing_L'||o.name==='bee_wing_R'
   const material=new THREE.MeshStandardMaterial({vertexColors:true,roughness:wing ? .32 : .48,metalness:wing ? .18 : .55,side:wing?THREE.DoubleSide:THREE.FrontSide,transparent:wing,opacity:wing ? .66 : 1,depthWrite:!wing})
   materials.add(material);o.material=material
   const attribute=o.geometry.getAttribute('color')
   if(attribute){const dark=new Float32Array(attribute.count*3),light=new Float32Array(attribute.count*3)
    const darkColors=palettes.dark.map(c=>new THREE.Color(c)),lightColors=palettes.light.map(c=>new THREE.Color(c))
    for(let i=0;i<attribute.count;i++){const r=attribute.getX(i),g=attribute.getY(i),b=attribute.getZ(i);const index=wing?3:r>g*1.3&&b<g*.3?0:r*.2126+g*.7152+b*.0722>.4?2:1;darkColors[index].toArray(dark,i*3);lightColors[index].toArray(light,i*3)}
    colors.push({attribute,dark,light})
   }
  })
  wingL=root.getObjectByName('bee_wing_L');wingR=root.getObjectByName('bee_wing_R')
  for(const name of ['bee_legs_front','bee_legs_L','bee_legs_R']){const leg=root.getObjectByName(name);if(leg)legs.push(leg)}
  // The prepared model's origin is the thorax. Lift it to center the whole silhouette.
  gltf.scene.position.y=.22;pose.add(gltf.scene);loaded=true;applyColors();resize();onState('ready');schedule()
 }).catch(()=>{if(!disposed){onState('fallback');renderer.domElement.style.visibility='hidden'}})
 return {setPaused(value:boolean){paused=value;host.dataset.motion=paused?'paused':'active';schedule()},dispose(){
  disposed=true;abort.abort();cancelAnimationFrame(frame);resizeObserver.disconnect();intersection.disconnect();themeObserver.disconnect()
  host.removeEventListener('pointermove',onPointer);host.removeEventListener('pointerleave',onLeave);document.removeEventListener('visibilitychange',onVisibility);renderer.domElement.removeEventListener('webglcontextlost',onLoss)
  geometries.forEach(g=>g.dispose());materials.forEach(m=>m.dispose());environmentMap.dispose();renderer.dispose();renderer.domElement.remove()
 }}
}
