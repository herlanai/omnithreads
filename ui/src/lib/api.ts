import {OmniWebClient} from './omniweb'
export type Agent={imageGeneration?:boolean;avatar?:string;language?:string;scheduleEnabled?:boolean;scheduleMinutes?:number;schedulePrompt?:string;scheduleTopic?:string;nextRunAt?:string;customModel?:boolean;model?:string;hasAppKey?:boolean;id:string;name:string;handle:string;role:string;bio:string;color:string;symbol:string;prompt:string;owned:boolean;followed:boolean}
export type Reference={postId:string;replyId?:string}
export type GeneratedImage={data:string;size:string}
export type Reply={images?:GeneratedImage[];parentId?:string;likes:number;liked:boolean;id:string;author:string;content:string;createdAt:string}
export type Post={images?:GeneratedImage[];quote?:Reference;id:string;author:string;content:string;topic:string;createdAt:string;likes:number;liked:boolean;saved:boolean;featured:boolean;artifact?:string;replies:Reply[];example:boolean}
export type Run={scheduled?:boolean;taskContent?:string;taskTopic?:string;autoReply?:boolean;id:string;postId:string;agentId:string;status:string;events:string[];createdAt:string}
export type Settings={imageModel?:string;hasArkKey?:boolean;language?:'zh-CN'|'en';theme?:'light'|'dark';model:string;hasAppKey:boolean;displayName:string}
export type State={agents:Agent[];posts:Post[];runs:Run[];settings:Settings}
export type Action={arkKey?:string;replyId?:string;quote?:Reference;kind:string;id?:string;content?:string;topic?:string;agent?:Partial<Agent>;settings?:Settings;appKey?:string}
type API={'community.state':{input:Record<string,never>;output:State};'community.action':{input:Action;output:{state:State;error?:string}}}
export const api=new OmniWebClient<API>()
