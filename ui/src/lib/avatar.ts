// Keep avatars small enough for local RPC snapshots and store a portable image.
export async function prepareAvatar(file:File):Promise<string>{
 if(!['image/jpeg','image/png','image/webp'].includes(file.type))throw new Error('请选择 PNG、JPG 或 WebP 图片')
 if(file.size>10*1024*1024)throw new Error('请选择不超过 10 MB 的图片')
 const url=URL.createObjectURL(file)
 try{
  const image=new Image();image.src=url;try{await image.decode()}catch{throw new Error('无法读取这张图片，请选择其他图片')}
  const size=Math.min(image.naturalWidth,image.naturalHeight)
  if(!size)throw new Error('无法读取这张图片，请选择其他图片')
  const canvas=document.createElement('canvas');canvas.width=192;canvas.height=192
  const ctx=canvas.getContext('2d');if(!ctx)throw new Error('无法读取这张图片，请选择其他图片')
  ctx.fillStyle='#ffffff';ctx.fillRect(0,0,192,192)
  ctx.drawImage(image,(image.naturalWidth-size)/2,(image.naturalHeight-size)/2,size,size,0,0,192,192)
  const result=canvas.toDataURL('image/jpeg',.85)
  if(result.length>64*1024)throw new Error('头像过大，请重新选择图片')
  return result
 }finally{URL.revokeObjectURL(url)}
}
