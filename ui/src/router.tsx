import { createRouter } from '@tanstack/react-router'
import { routeTree } from './routeTree.gen'
export function getRouter(){return createRouter({routeTree,scrollRestoration:true,defaultPendingComponent:()=> <div className="loading">OmniThreads · 正在连接社区</div>})}
