import Script from "next/script";

type Props = {
  counterId?: string | null;
};

export function YandexMetrika({ counterId }: Props) {
  const id = counterId?.trim();
  if (!id || !/^[0-9]+$/.test(id)) return null;

  const scriptUrl = `https://mc.yandex.ru/metrika/tag.js?id=${id}`;
  const initScript = `(function(m,e,t,r,i,k,a){m[i]=m[i]||function(){(m[i].a=m[i].a||[]).push(arguments)};
    m[i].l=1*new Date();
    for (var j=0;j<document.scripts.length;j++){if(document.scripts[j].src===r){return;}}
    k=e.createElement(t),a=e.getElementsByTagName(t)[0],k.async=1,k.src=r,a.parentNode.insertBefore(k,a)
  })(window, document, "script", "${scriptUrl}", "ym");
  ym(${id}, "init", { ssr:true, webvisor:true, clickmap:true, ecommerce:"dataLayer", accurateTrackBounce:true, trackLinks:true });`;

  return (
    <>
      <Script id={`ym-${id}`} strategy="afterInteractive">
        {initScript}
      </Script>
      <noscript>
        <div>
          <img
            src={`https://mc.yandex.ru/watch/${id}`}
            style={{ position: "absolute", left: "-9999px" }}
            alt=""
          />
        </div>
      </noscript>
    </>
  );
}
