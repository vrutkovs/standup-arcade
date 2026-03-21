module meet-attendees

go 1.22.2

require golang.org/x/oauth2 v0.24.0

require cloud.google.com/go/compute/metadata v0.3.0 // indirect

replace golang.org/x/oauth2 => github.com/golang/oauth2 v0.24.0

replace cloud.google.com/go/compute/metadata => github.com/googleapis/google-cloud-go/compute/metadata v0.3.0
