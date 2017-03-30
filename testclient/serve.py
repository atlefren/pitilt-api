import requests
import json
import time
KEY = 'ABCD'

URL = 'http://localhost:8080'


def send_measurement(name, value, name2, value2):
    data = [
        {
            'name': name,
            'type': 'temperature',
            'value': value,
            'timestamp': int(time.time())
        },
        {
            'name': name2,
            'value': value2,
            'type': 'gravity',
            'timestamp': int(time.time())
        }
    ]
    r = requests.post('%s/data' % URL, data=json.dumps(data), headers={'X-PYTILT-KEY': KEY})
    print r


if __name__ == '__main__':
    #send_measurement('temp', 1, 'gravity', 2)

    while True:
        send_measurement('temp', 1, 'gravity', 2)
        time.sleep(10)
   
