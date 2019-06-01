var LoginView = {
    html: 'views/Login.html',
    login: function () {
        L(this.data.token)
        var token = sha1('SSGO-' + this.data.token + '-Deploy')
        http.post('/login', {accessToken: token}).then(function (data) {
            if (data > 0) {
                http.upHeaders['Access-Token'] = token
                states.set({logined: true, authLevel: data})
                sessionStorage.accessToken = token
            } else {
                states.set({logined: false, error: data || 'Bad Token'})
            }
        }).catch(function (err) {
            states.set({logined: false, error: err})
        })
    },

    logout: function () {
        delete sessionStorage.accessToken
        states.set('logined', false)
    }
}
