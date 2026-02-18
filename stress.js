import http from "k6/http"
import { check, sleep } from "k6"

export const options = {
    vus: 10,
    duration: '10s'
}

export default function () {
    const params = {
        headers: {
            'X-User-ID': "user69",
        }
    }

    const res = http.get('http://localhost:8081', params)

    check(res, {
        'is status 200': (r) => r.status === 200,
        'has rate-limit header': (r) => r.headers['X-Ratelimit-Limit'] !== undefined,
    });

    sleep(0.1)
}


