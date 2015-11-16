# beacon-go
AppEngine implementation of the [Beacon API](http://ga4gh.org/#/beacon) from the Global Alliance for Genomics and Health written in Go.

Here is an example query that is running against a private copy (for demonstration purposes) of the [Illumina Platinum Genomes](http://googlegenomics.readthedocs.org/en/latest/use_cases/discover_public_data/platinum_genomes.html) data:

> [http://goapp-beacon.appspot.com/?chromosome=chr17&coordinate=41196407&allele=A](http://goapp-beacon.appspot.com/?chromosome=chr17&coordinate=41196407&allele=A).

## Prerequisites

In order to setup and deploy this application, you will need:

- A Google Developers [project](https://developers.google.com/console/help/new/) with
  - [Billing](https://developers.google.com/console/help/new/#billing) enabled
  - Access to the Genomics [APIs](https://developers.google.com/console/help/new/#activating-and-deactivating-apis)
- The [App Engine SDK](https://cloud.google.com/appengine/downloads) for the Go programming language.

## Setup
- Prepare Genomics data.
  - Follow [this guide](https://cloud.google.com/genomics/v1/load-variants) to upload genomics data to a Google Cloud Project.
- Clone this repo.
  - `git clone git@github.com:googlegenomics/beacon-go.git`
- Edit the configuration.
  - In `beacon.go`, edit the value of `variantSetIds` to reference your data.
- Deploy.
  - `goapp deploy <your project>`
- Query your new Beacon.
  - e.g. `http://<your project>.appspot.com/?chromosome=chr17&coordinate=41196407&allele=A`
  - Note that the `chromosome` parameter might look like `chr17` or just plain `17` depending on the reference used.


## Authentication

This application uses the [Application Default Credentials](https://developers.google.com/identity/protocols/application-default-credentials).  That means that, by default, it will have access to all data within the project in which it is deployed.

It is also possible to grant the application access to data within other projects.  To accomplish this, add the "App Engine Service Account" of the project in which the application is deployed (found under the "Permissions" section in the [Google Developers Console](https://console.developers.google.com)) as a member of the project that contains the data.
