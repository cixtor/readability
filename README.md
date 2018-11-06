# Readability

Readability is a library written in Go (golang) to parse, analyze and convert HTML pages into readable content. Originally an Arc90 Experiment, it is now incorporated into Safari’s Reader View.

> Despite the ubiquity of reading on the web, readers remain a neglected audience. Much of our talk about web design revolves around a sense of movement: users are thought to be finding, searching, skimming, looking. We measure how frequently they click but not how long they stay on the page. We concern ourselves with their travel and participation–how they move from page to page, who they talk to when they get there–but forget the needs of those whose purpose is to be still. Readers flourish when they have space–some distance from the hubbub of the crowds–and as web designers, there is yet much we can do to help them carve out that space.
>
> [In Defense Of Readers](http://alistapart.com/articles/indefenseofreaders), by [Mandy Brown](http://www.aworkinglibrary.com/)

## Evolution of Readability Web Engines

| Product | Year | Shutdown |
|---------|------|----------|
| [Instapaper](https://www.instapaper.com/) | 2008 | N/A |
| [Arc90 Readability](https://code.google.com/archive/p/arc90labs-readability/) | 2009 | [Sep 30, 2016](https://medium.com/@readability/the-readability-bookmarking-service-will-shut-down-on-september-30-2016-1641cc18e02b) |
| [Apple Readability](https://developer.apple.com/documentation/safariextensions/safarireader) | 2010 | N/A |
| [Microsoft Reading View](https://docs.microsoft.com/en-us/microsoft-edge/dev-guide/browser-features/reading-view) | 2014 | N/A |
| [Mozilla Readability](https://github.com/mozilla/readability) | 2015 | N/A |
| [Mercury Reader](https://mercury.postlight.com/) | 2016 | [Apr 15, 2019](https://www.reddit.com/r/mac/comments/apkhzs/a/) |

## Reader Mode Parser Diversity

All modern web browsers, except for Google Chrome, include an option to parse, analyze, and extract the main content from web pages to provide what is commonly known as “Reading Mode”. Reading Mode is a separate web rendering mode that strips out repeated and irrelevant content, this allows the web browser to extract the main content and display it cleanly and consistently to the user.

| Vendor | Product | Parser | Environments |
|--------|---------|--------|--------------|
| Mozilla | Firefox | Mozilla Readability | Desktop and Android |
| GNOME | Web | Mozilla Readability | Desktop |
| Vivaldi | Vivaldi | Mozilla Readability | Desktop |
| Yandex | Browser | Mozilla Readability | Desktop |
| Samsung | Browser | Mozilla Readability | Android |
| Apple | Safari | Safari Reader | macOS and iOS |
| Maxthon | Maxthon | Maxthon Reader | Desktop |
| Microsoft | Edge | EdgeHTML | Windows and Windows Mobile |
| Microsoft | Edge Mobile | Chrome DOM Distiller | Android |
| Google | Chrome | Chrome DOM Distiller | Android |
| Postlight | Mercury Reader | Web Reader | Web / browser extension |
| Instant Paper | Instapaper | Instaparser | Web / browser extension |
| Mozilla | Pocket | Unknown | Web / browser extension |

---

Ref: https://web.archive.org/web/20150817073201/http://lab.arc90.com/2009/03/02/readability/
