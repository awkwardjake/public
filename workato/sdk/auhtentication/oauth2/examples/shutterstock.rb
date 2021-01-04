{
  title: "shutterstock",
  connection: {
    fields: [
      ## oauth2 ##
      {
        name: "client_id",
        optional: false,
      },
      {
        name: "client_secret",
        control_type: :password,
        optional: false,
      },
    ## /oauth2 ##
    ],
    authorization: {
      ### oauth2 ###
      type: "oauth2",
      authorization_url: ->(connection) {
        scopes = [
          "user.view",
          "user.edit",
          "collections.view",
          "collections.edit",
          "licenses.view",
          "licenses.create",
          "earnings.view",
          "media.upload",
          "media.submit",
          "media.edit",
          "purchases.view",
          "reseller.view",
          "reseller.purchase",
        ].join(" ")
        params = {
          client_id: connection["client_id"],
          redirect_uri: "https://www.workato.com/oauth/callback",
          response_type: "code",
          scope: scopes,
          state: "",
        }.to_param
        "https://api.shutterstock.com/v2/oauth/authorize?" + params
      },
      acquire: ->(connection, auth_code) {
        payload = {
          client_id: connection["client_id"],
          client_secret: connection["client_secret"],
          grant_type: "authorization_code",
          expires: false,
          code: auth_code,
        }
        response = post("https://api.shutterstock.com/v2/oauth/access_token")
          .payload(payload)
          .request_format_www_form_urlencoded
        {
          access_token: response["access_token"],
        }
      },
      apply: ->(connection, access_token) {
        headers("Authorization": "Bearer #{access_token}")
      },
    ### /oauth2 ###
    },
    base_uri: ->(connection) {
      "https://api.shutterstock.com"
    },
  },
  test: ->(connection) {
    get("https://api.shutterstock.com/v2/test?text=helloWorkato")
  },
}
