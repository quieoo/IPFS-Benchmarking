//import { create } from 'ipfs-http-client'
const {create} =require('ipfs-http-client');
var fs = require("fs");
// connect to the default API address http://localhost:5001
const client = create()

// connect to a different API
//const client = create('http://127.0.0.1:5002')

// connect using a URL
//const client = create(new URL('http://127.0.0.1:5002'))

// call Core API methods
var func=async(callback)=>{
    const data=fs.readFileSync('t', function (err, data) {
        if (err) {
            return console.error(err);
        }
        return data.toString()
     });
     const cid=await client.add(data)
     callback(cid)
    }

const myFunc=async()=>{
    const data=fs.readFileSync('t', function (err, data) {
        if (err) {
            return console.error(err);
        }
        return data.toString()
     });

    //console.log(data)
    const { cid } = await client.add(data)
    return cid;
}

func(function(cid){
    console.log(cid)
})