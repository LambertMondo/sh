import os
import asyncio
from aiohttp import web, ClientSession, ClientTimeout
import ssl

TARGET_HOST = os.environ.get("TARGET_HOST", "konoyves.shop")
TARGET_PORT = int(os.environ.get("TARGET_PORT", "443"))

async def proxy_handler(request):
    url = f"https://{TARGET_HOST}:{TARGET_PORT}{request.path_qs}"
    
    headers = dict(request.headers)
    headers.pop("host", None)
    
    ssl_context = ssl.create_default_context()
    ssl_context.check_hostname = False
    ssl_context.verify_mode = ssl.CERT_NONE
    
    timeout = ClientTimeout(total=30)
    async with ClientSession(timeout=timeout) as session:
        try:
            async with session.request(
                method=request.method,
                url=url,
                headers=headers,
                data=await request.read() if request.can_read_body else None,
                ssl=ssl_context,
                allow_redirects=False
            ) as resp:
                body = await resp.read()
                response_headers = dict(resp.headers)
                response_headers.pop("content-encoding", None)
                response_headers.pop("transfer-encoding", None)
                
                return web.Response(
                    body=body,
                    status=resp.status,
                    headers=response_headers
                )
        except Exception as e:
            return web.Response(text=f"Proxy error: {e}", status=502)

app = web.Application()
app.router.add_route("*", "/{path:.*}", proxy_handler)

if __name__ == "__main__":
    port = int(os.environ.get("PORT", 8080))
    web.run_app(app, host="0.0.0.0", port=port)