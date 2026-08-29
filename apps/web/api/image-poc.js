export default async function handler(req, res) {
  if (req.method !== 'POST') return res.status(405).json({error:'POST only'});
  const key = process.env.OPENAI_API_KEY;
  if (!key) return res.status(503).json({error:'OPENAI_API_KEY is not configured on the web deployment'});
  const raw = typeof req.body?.prompt === 'string' ? req.body.prompt.trim() : '';
  if (!raw || raw.length > 1000) return res.status(400).json({error:'prompt must be 1..1000 characters'});

  const prompt = `Create a single non-explicit image suitable as source material for a fictional Japanese personal-computer BBS file library circa 1996. All depicted people must be adults age 20 or older. No nudity or explicit sexual content. Do not add text unless the subject requires it. Subject requested by the user: ${raw}`;
  try {
    const r = await fetch('https://api.openai.com/v1/images/generations', {
      method:'POST',
      headers:{'Authorization':`Bearer ${key}`,'Content-Type':'application/json'},
      body:JSON.stringify({model:process.env.OPENAI_IMAGE_MODEL || 'gpt-image-1',prompt,size:'1024x1024',quality:'low',n:1})
    });
    const data = await r.json();
    if (!r.ok) return res.status(r.status).json({error:data?.error?.message || 'OpenAI image generation failed'});
    const item=data?.data?.[0];
    const image=item?.b64_json ? `data:image/png;base64,${item.b64_json}` : item?.url;
    if (!image) return res.status(502).json({error:'OpenAI returned no image'});
    res.status(200).json({image,model:process.env.OPENAI_IMAGE_MODEL || 'gpt-image-1'});
  } catch (e) {
    res.status(502).json({error:e instanceof Error ? e.message : String(e)});
  }
}
