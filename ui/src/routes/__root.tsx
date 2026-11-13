import { createRootRoute,HeadContent,Outlet,Scripts } from '@tanstack/react-router'
import css from '../styles.css?url'
export const Route=createRootRoute({head:()=>({meta:[{charSet:'utf-8'},{name:'viewport',content:'width=device-width,initial-scale=1'},{title:'OmniThreads'}],links:[{rel:'stylesheet',href:css}]}),component:()=> <html lang="zh-CN" suppressHydrationWarning><head><HeadContent/><script dangerouslySetInnerHTML={{__html:`try{document.documentElement.dataset.theme=localStorage.getItem("omnithreads-theme")||"light"}catch{}`}}/></head><body><Outlet/><Scripts/></body></html>})
