// 选择器
window.$ = function (selector) {
    if (!selector) return document.body
    return document.querySelector(selector)
}
window.$$ = function (selector) {
    if (!selector) return [document.body]
    return document.querySelectorAll(selector)
}
window.L = function (obj) {
    console.log(obj)
}
window.CP = function (data) {
    return JSON.parse(JSON.stringify(data))
}

var states = new svcState.State('binds')
var http = new svcWeb.Http('//' + location.host)
var route = new svcWeb.Route(states)
var tpl = new svcWeb.Tpl()

// 设置根路由Root
route.Root = {
    getSubView: function (subName) {
        switch (subName) {
            case 'login':
                return LoginView
            case 'deploy':
                return DeployView
        }
    }
}

var startRoute = location.hash ? location.hash.substring(1) : ''
route.bindHash()
states.bind('logined', function (data) {
    if (data.logined) {
        if (!startRoute || /^\/login/.test(startRoute)) {
            route.go('/deploy/global')
        } else {
            route.go(startRoute)
        }
    } else {
        route.go('/login')
    }
})

window.addEventListener('load', function () {
    if(!sessionStorage.accessToken){
        states.set({logined: false})
        return
    }
    http.post("/login", {accessToken: sessionStorage.accessToken}).then(function (data) {
        if (data > 0) {
            http.upHeaders['Access-Token'] = sessionStorage.accessToken
            states.set({logined: true, authLevel: data})
        } else {
            states.set({logined: false, error: data || 'Bad Token'})
        }
    })
})
